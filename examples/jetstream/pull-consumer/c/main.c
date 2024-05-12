#include <stdio.h>
#include <nats.h>

int main()
{
    natsStatus s = NATS_OK;
    natsOptions *opts = NULL;
    natsConnection *nc = NULL;
    natsSubscription *sub = NULL;

    jsOptions jsOpts;
    jsCtx *js = NULL;
    jsStreamConfig cfg;
    jsSubOptions so;
    jsStreamInfo *si = NULL;
    jsErrCode jerr = 0;

    natsMsgList list = {0};
    int c, i, ibatch;

    // Use the env variable if running in the container, otherwise use the
    // default.
    const char *url = getenv("NATS_URL");
    s = natsOptions_Create(&opts);
    if (s == NATS_OK && url != NULL)
    {
        s = natsOptions_SetURL(opts, url);
    }

    // Create an unauthenticated connection to NATS.
    if (s == NATS_OK)
        s = natsConnection_Connect(&nc, opts);

    // Access JetStream for managing streams and consumers as well as for
    // publishing and consuming messages to and from the stream.
    if (s == NATS_OK)
        s = jsOptions_Init(&jsOpts);
    if (s == NATS_OK)
        s = natsConnection_JetStream(&js, nc, &jsOpts);

    // Add a simple limits-based stream.
    if (s == NATS_OK)
        s = jsStreamConfig_Init(&cfg);
    if (s == NATS_OK)
    {
        cfg.Name = "EVENTS";
        cfg.Subjects = (const char *[1]){"event.>"};
        cfg.SubjectsLen = 1;
        s = js_AddStream(&si, js, &cfg, &jsOpts, &jerr);
    }

    // Publish a few messages for the example.
    if (s == NATS_OK)
        printf("Publish 3 messages for the example\n");
    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.1", "0123456789", 10, NULL, &jerr);
    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.2", "0123456789", 10, NULL, &jerr);
    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.3", "0123456789", 10, NULL, &jerr);

    // Create a pull consumer subscription bound to the previously created
    // stream. If durable name is not supplied, consumer will be removed after
    // InactiveThreshold (defaults to 5 seconds) is reached when not actively
    // consuming messages. `Name` is optional, if not provided it will be
    // auto-generated. For this example, let's use the consumer with no options,
    // which will be ephemeral with auto-generated name.
    if (s == NATS_OK)
        s = jsSubOptions_Init(&so);
    if (s == NATS_OK)
    {
        printf("Create a pull consumer and use natsSubscription_Fetch to receive messages\n");
        so.Stream = "EVENTS";
        s = js_PullSubscribe(&sub, js, "event.>", NULL, &jsOpts, &so, &jerr);
    }

    // Use natsSubscription_Fetch to fetch the messages. Here we attempt to
    // fetch a batch of up to 2 messages with a 5 second timeout, and we stop
    // trying once the expected 3 messages are successfully fetched.
    //
    // **Note**: natsSubscription_Fetch will not wait for the timeout while we are
    // fetching pre-buffered messages. The response time is in single ms.
    //
    // **Note**: each fetched message must be acknowledged.
    for (ibatch = 0, c = 0; (s == NATS_OK) && (c < 3); ibatch++)
    {
        int64_t start = nats_Now();
        s = natsSubscription_Fetch(&list, sub, 2, 5000, &jerr);
        if (s == NATS_OK)
        {
            c += (int64_t)list.Count;
            printf("natsSubscription_Fetch: batch %d of %d messages in %ldms\n", ibatch, list.Count, nats_Now() - start);
        }
        else
        {
            printf("natsSubscription_Fetch error: %d:\n", s);
            nats_PrintLastErrorStack(stderr);
        }
        for (i = 0; (s == NATS_OK) && (i < list.Count); i++)
        {
            s = natsMsg_Ack(list.Msgs[i], &jsOpts);
            if (s == NATS_OK)
                printf("received and acked message on %s\n", natsMsg_GetSubject(list.Msgs[i]));
        }
        natsMsgList_Destroy(&list);
    }

    // Attempt to fetch more messages, but this time we will wait for the
    // timeout since there are no more pre-buffered messages.
    if (s == NATS_OK)
    {
        int64_t start = nats_Now();
        s = natsSubscription_Fetch(&list, sub, 2, 500, &jerr);
        printf("extra natsSubscription_Fetch returned status %d and %d messages in %ldms\n", s, list.Count, nats_Now() - start);
        s = NATS_OK;
    }

    // This consumer will be deleted automatically, but need to free the sub.
    natsSubscription_Destroy(sub);
    sub = NULL;

    // Create another similar pull consumer, same scenario but using
    // `natsSubscription_FetchRequest` for precise control.
    if (s == NATS_OK)
    {
        printf("Create another pull consumer and use natsSubscription_FetchRequest to receive messages\n");
        so.Stream = "EVENTS";
        s = js_PullSubscribe(&sub, js, "event.>", NULL, &jsOpts, &so, &jerr);
    }

    // Use `natsSubscription_FetchRequest` to fetch the messages.
    //
    // We set the batch size to 1000, but MaxBytes of 200 so we will only get 2
    // messages at a time. Note that since we do not set NoWait, the call will
    // block until the batch is filled or the timeout expires, unlike
    // `natsSubscription_Fetch`.
    for (ibatch = 0, c = 0; (s == NATS_OK) && (c < 3); ibatch++)
    {
        int64_t start = nats_Now();
        jsFetchRequest fr = {
            .Batch = 1000,
            // .NoWait = true,
            .Expires = 500 * 1000 * 1000,
            .MaxBytes = 200,
        };
        s = natsSubscription_FetchRequest(&list, sub, &fr);
        if (s == NATS_OK)
        {
            c += (int64_t)list.Count;
            printf("natsSubscription_FetchRequest: batch %d of %d messages in %ldms\n", ibatch, list.Count, nats_Now() - start);
        }
        else
        {
            printf("FetchRequest error: %d:\n", s);
            nats_PrintLastErrorStack(stderr);
        }
        for (i = 0; (s == NATS_OK) && (i < list.Count); i++)
        {
            s = natsMsg_Ack(list.Msgs[i], &jsOpts);
            if (s == NATS_OK)
                printf("received and acked message on %s\n", natsMsg_GetSubject(list.Msgs[i]));
        }
        natsMsgList_Destroy(&list);
    }

    // Clean up the consumer by unsubscribing.
    natsSubscription_Destroy(sub);
    sub = NULL;

    // Finally, create a durable pull consumer explicitly, and bind a
    // subscription to it more than once. 
    // js_AddConsumer(&ci, js, "EVENTS", cfg, &jsOpts, &jerr);

    jsCtx_Destroy(js);
    natsConnection_Destroy(nc);
    natsOptions_Destroy(opts);
    jsStreamInfo_Destroy(si);

    if (s != NATS_OK)
    {
        nats_PrintLastErrorStack(stderr);
    }
    return 0;
}