#include <stdio.h>
#include <nats.h>

#define STREAM_NAME "EVENTS"
#define CONSUMER_NAME "event-consumer"
#define SUBSCRIBE_SUBJECT "event.>"
#define SUBJECT_PREFIX "event."
#define NUM_MESSAGES 5

static natsStatus publishTestMessages(jsCtx *js);
static natsStatus init(natsConnection **newnc, jsCtx **newjs, jsOptions *jsOpts);
static natsStatus exampleFetch(jsCtx *js, jsOptions *jsOpts);
static natsStatus exampleFetchRequest(jsCtx *js, jsOptions *jsOpts);
static natsStatus exampleNamedConsumer(jsCtx *js, jsOptions *jsOpts);

typedef natsStatus (*examplef)(jsCtx *js, jsOptions *jsOpts);

int main()
{
    natsStatus s = NATS_OK;
    natsConnection *nc = NULL;
    jsOptions jsOpts;
    jsCtx *js = NULL;
    examplef examples[] = {exampleFetch, exampleFetchRequest, exampleNamedConsumer};
    int i;
    int N = sizeof(examples) / sizeof(examples[0]);

    // Initialize the NATS connection and JetStream context.
    s = init(&nc, &js, &jsOpts);

    // Run the examples.
    for (i = 0; (i < N) && (s == NATS_OK); i++)
    {
        examplef f = examples[i];
        s = f(js, &jsOpts);
    }

    // Cleanup and finish.
    jsCtx_Destroy(js);
    natsConnection_Destroy(nc);
    if (s != NATS_OK)
    {
        nats_PrintLastErrorStack(stderr);
        return 1;
    }
    return 0;
}

// Initialize the NATS connection and JetStream context, publish NUM_MESSAGES
// test messages.
static natsStatus init(natsConnection **newnc, jsCtx **newjs, jsOptions *jsOpts)
{
    natsStatus s = NATS_OK;
    natsOptions *opts = NULL;
    natsConnection *nc = NULL;

    jsCtx *js = NULL;
    jsStreamConfig cfg;
    jsStreamInfo *si = NULL;
    jsErrCode jerr = 0;
    char subject[] = SUBJECT_PREFIX "99999999999999";
    int i;

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
    natsOptions_Destroy(opts);

    // Access JetStream for managing streams and consumers as well as for
    // publishing and consuming messages to and from the stream.
    if (s == NATS_OK)
        s = jsOptions_Init(jsOpts);
    if (s == NATS_OK)
        s = natsConnection_JetStream(&js, nc, jsOpts);

    // Add a simple limits-based stream.
    if (s == NATS_OK)
        s = jsStreamConfig_Init(&cfg);
    if (s == NATS_OK)
    {
        cfg.Name = STREAM_NAME;
        cfg.Subjects = (const char *[1]){SUBSCRIBE_SUBJECT};
        cfg.SubjectsLen = 1;
        s = js_AddStream(&si, js, &cfg, jsOpts, &jerr);
        jsStreamInfo_Destroy(si);
    }
    if (s == NATS_OK)
        printf("Created a stream named '%s' with 1 subject '%s'\n", STREAM_NAME, SUBSCRIBE_SUBJECT);

    // Publish NUM_MESSAGES messages for the examples.
    for (i = 0; (s == NATS_OK) && (i < NUM_MESSAGES); i++)
    {
        sprintf(subject, "%s%d", SUBJECT_PREFIX, i + 1);
        s = js_Publish(NULL, js, subject, "01234567890123456789012345678901234567890123456789", 50, NULL, &jerr);
    }
    if (s == NATS_OK)
        printf("Published %d messages for the example\n", NUM_MESSAGES);

    if (s == NATS_OK)
    {
        *newnc = nc;
        *newjs = js;
    }
    else
    {
        jsCtx_Destroy(js);
        natsConnection_Destroy(nc);
    }

    return s;
}

// Create a pull consumer subscription and use `natsSubscription_Fetch` to
// receive messages.
static natsStatus exampleFetch(jsCtx *js, jsOptions *jsOpts)
{
    natsStatus s = NATS_OK;
    natsSubscription *sub = NULL;
    jsErrCode jerr = 0;
    natsMsgList list = {0};
    jsSubOptions so;
    int c, i, ibatch;

    // Create a pull consumer subscription. The durable name (4th parameter) is
    // not supplied, so the consumer will be removed after `InactiveThreshold`
    // (defaults to 5 seconds) is reached when not actively consuming messages.
    s = jsSubOptions_Init(&so);
    if (s == NATS_OK)
    {
        printf("exampleFetch: create a pull consumer and use natsSubscription_Fetch to receive messages\n");
        so.Stream = STREAM_NAME;
        s = js_PullSubscribe(&sub, js, SUBSCRIBE_SUBJECT, NULL, jsOpts, &so, &jerr);
    }

    // Use `natsSubscription_Fetch` to fetch the messages. Here we attempt to
    // fetch a batch of up to 2 messages with a 5 second timeout, and we stop
    // trying once the expected NUM_MESSAGES messages are successfully fetched.
    //
    // **Note**: `natsSubscription_Fetch` will not wait for the timeout while we
    // are fetching pre-buffered messages. The response time is in single ms.
    //
    // **Note**: each fetched message must be acknowledged.
    //
    // **Note**: `natsMsgList_Destroy` will destroy the fetched messages.
    for (ibatch = 0, c = 0; (s == NATS_OK) && (c < NUM_MESSAGES); ibatch++)
    {
        int64_t start = nats_Now();
        s = natsSubscription_Fetch(&list, sub, 2, 5000, &jerr);
        if (s == NATS_OK)
        {
            c += (int64_t)list.Count;
            printf("exampleFetch: batch #%d (%d messages) in %dms\n", ibatch, list.Count, (int)(nats_Now() - start));
        }
        else
        {
            printf("exampleFetch: error: %d:\n", s);
            nats_PrintLastErrorStack(stderr);
        }
        for (i = 0; (s == NATS_OK) && (i < list.Count); i++)
        {
            s = natsMsg_Ack(list.Msgs[i], jsOpts);
            printf("exampleFetch: received and acked message on %s\n", natsMsg_GetSubject(list.Msgs[i]));
        }
        natsMsgList_Destroy(&list);
    }

    // Attempt to fetch more messages, but this time we will wait for the
    // 500ms timeout since there are no more pre-buffered messages.
    if (s == NATS_OK)
    {
        int64_t start = nats_Now();
        s = natsSubscription_Fetch(&list, sub, 2, 500, &jerr);
        printf("exampleFetch: extra natsSubscription_Fetch returned status %d and %d messages in %dms\n",
               s, list.Count, (int)(nats_Now() - start));
        s = NATS_OK;
    }

    // Cleanup.
    natsSubscription_Destroy(sub);

    return s;
}

// Create another similar pull consumer subscription and use
// `natsSubscription_FetchRequest` to receive messages with more precise
// control.
static natsStatus exampleFetchRequest(jsCtx *js, jsOptions *jsOpts)
{
    natsStatus s = NATS_OK;
    natsSubscription *sub = NULL;
    jsErrCode jerr = 0;
    natsMsgList list = {0};
    jsSubOptions so;
    int c, i, ibatch;

    // Create another similar pull consumer, same scenario but using
    // `natsSubscription_FetchRequest` for precise control.
    s = jsSubOptions_Init(&so);
    if (s == NATS_OK)
    {
        printf("exampleFetchRequest: create pull consumer and use natsSubscription_FetchRequest to receive messages\n");
        so.Stream = STREAM_NAME;
        s = js_PullSubscribe(&sub, js, SUBSCRIBE_SUBJECT, NULL, jsOpts, &so, &jerr);
    }

    // Use `natsSubscription_FetchRequest` to fetch the messages. We set the
    // batch size to 1000, but MaxBytes of 300 so we will only get 2 messages at
    // a time.
    //
    // **Note**: Setting `.NoWait` causes the request to return as soon as there
    // are some messages availabe, not necessarily the entire batch. By default,
    // we wait `.Expires` time if there are not enough messages to make a full
    // batch.
    for (ibatch = 0, c = 0; (s == NATS_OK) && (c < NUM_MESSAGES); ibatch++)
    {
        int64_t start = nats_Now();
        jsFetchRequest fr = {
            .Batch = 1000,
            /* .NoWait = true, */
            .Expires = 500 * 1000 * 1000,
            .MaxBytes = 300,
        };
        s = natsSubscription_FetchRequest(&list, sub, &fr);
        if (s == NATS_OK)
        {
            c += (int64_t)list.Count;
            printf("exampleFetchRequest: batch #%d (%d messages) in %dms\n", ibatch, list.Count, (int)(nats_Now() - start));
        }
        else
        {
            printf("exampleFetchRequest: error: %d:\n", s);
            nats_PrintLastErrorStack(stderr);
        }
        for (i = 0; (s == NATS_OK) && (i < list.Count); i++)
        {
            s = natsMsg_Ack(list.Msgs[i], jsOpts);
            if (s == NATS_OK)
                printf("exampleFetchRequest: received and acked message on %s\n", natsMsg_GetSubject(list.Msgs[i]));
        }
        natsMsgList_Destroy(&list);
    }

    natsSubscription_Destroy(sub);

    return s;
}

// Create a pull consumer, then bind 2 subscriptions to it.
static natsStatus exampleNamedConsumer(jsCtx *js, jsOptions *jsOpts)
{
    natsStatus s = NATS_OK;
    jsConsumerInfo *ci = NULL;
    jsConsumerConfig cfg;
    natsSubscription *sub1 = NULL;
    natsSubscription *sub2 = NULL;
    jsErrCode jerr = 0;
    natsMsgList list = {0};
    jsSubOptions so;
    int i;
    bool done = false;

    jsConsumerConfig_Init(&cfg);
    cfg.Name = CONSUMER_NAME;
    s = js_AddConsumer(&ci, js, STREAM_NAME, &cfg, jsOpts, &jerr);
    if (s == NATS_OK)
    {
        printf("exampleNamedConsumer: create a pull consumer named '%s'\n", CONSUMER_NAME);
    }
    jsConsumerInfo_Destroy(ci);

    // Create a named pull consumer explicitly and subscribe to it twice.
    //
    // **Note**: no delivery subject in `js_PullSubsccribe` since it will bind
    // to the consumer by name.
    //
    // **Note**: subscriptions are "balanced" in that each message is processed
    // by one or the other.
    if (s == NATS_OK)
        s = jsSubOptions_Init(&so);

    if (s == NATS_OK)
    {
        printf("exampleNamedConsumer: bind 2 subscriptions to the consumer\n");
        so.Stream = STREAM_NAME;
        so.Consumer = CONSUMER_NAME;
        s = js_PullSubscribe(&sub1, js, NULL, NULL, jsOpts, &so, &jerr);
    }
    if (s == NATS_OK)
        s = js_PullSubscribe(&sub2, js, NULL, NULL, jsOpts, &so, &jerr);

    int64_t start = nats_Now();
    for (i = 0; (s == NATS_OK) && (!done); i++)
    {
        natsSubscription *sub = (i % 2 == 0) ? sub1 : sub2;
        const char *name = (i % 2 == 0) ? "sub1" : "sub2";
        start = nats_Now();

        s = natsSubscription_Fetch(&list, sub, 1, 100, &jerr);
        if ((s == NATS_OK) && (list.Count == 1))
        {
            printf("exampleNamedConsumer: fetched from %s subject '%s' in %dms\n",
                   name, natsMsg_GetSubject(list.Msgs[0]), (int)(nats_Now() - start));
            s = natsMsg_Ack(list.Msgs[0], jsOpts);
        }
        else if ((s == NATS_OK) && (list.Count != 1))
        {
            printf("exampleNamedConsumer: fetched wrong number of messages from %s: expected 1, got %d\n", name, list.Count);
            s = NATS_ERR;
        }
        else if (s == NATS_TIMEOUT && list.Count == 0)
        {
            printf("exampleNamedConsumer: got NATS_TIMEOUT from %s in %dms, no more messages for now\n", name, (int)(nats_Now() - start));
            s = NATS_OK;
            done = true;
        }
        else
        {
            printf("exampleNamedConsumer: error: %d:\n", s);
            nats_PrintLastErrorStack(stderr);
        }

        natsMsgList_Destroy(&list);
    }

    // Cleanup.
    natsSubscription_Drain(sub1);
    natsSubscription_Drain(sub2);
    natsSubscription_Destroy(sub1);
    natsSubscription_Destroy(sub2);

    s = js_DeleteConsumer(js, STREAM_NAME, CONSUMER_NAME, jsOpts, &jerr);
    if (s == NATS_OK)
    {
        printf("exampleNamedConsumer: deleted consumer '%s'\n", CONSUMER_NAME);
    }

    return s;
}