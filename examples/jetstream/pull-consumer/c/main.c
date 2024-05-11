#include <stdio.h>
#include <nats/nats.h>

int main()
{
    natsStatus s = NATS_OK;
    natsOptions *opts = NULL;
    natsConnection *nc = NULL;
    natsSubscription *sub = NULL;
    natsMsg *msg = NULL;
    jsOptions jsOpts;
    jsCtx *js = NULL;
    jsStreamConfig cfg;
    jsSubOptions so;
    jsStreamInfo *si = NULL;
    jsErrCode jerr = 0;
    natsMsgList list = {0};
    int ifetch, iack;

    const char *url = getenv("NATS_URL");
    s = natsOptions_Create(&opts);
    if (s == NATS_OK && url != NULL)
    {
        s = natsOptions_SetURL(opts, url);
    }

    if (s == NATS_OK)
        s = natsConnection_Connect(&nc, opts);
    if (s == NATS_OK)
        s = jsOptions_Init(&jsOpts);
    if (s == NATS_OK)
        s = natsConnection_JetStream(&js, nc, &jsOpts);

    if (s == NATS_OK)
        s = jsStreamConfig_Init(&cfg);
    if (s == NATS_OK)
    {
        cfg.Name = "EVENTS";
        cfg.Subjects = (const char *[1]){"event.>"};
        cfg.SubjectsLen = 1;
        s = js_AddStream(&si, js, &cfg, &jsOpts, &jerr);
    }

    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.1", NULL, 0, NULL, &jerr);
    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.2", NULL, 0, NULL, &jerr);
    if (s == NATS_OK)
        s = js_Publish(NULL, js, "event.3", NULL, 0, NULL, &jerr);

    if (s == NATS_OK)
        s = jsSubOptions_Init(&so);
    if (s == NATS_OK)
    {
        so.Stream = "EVENTS";
        s = js_PullSubscribe(&sub, js, "event.>", NULL, &jsOpts, &so, &jerr);
    }

    for (ifetch = 0; (s == NATS_OK) && (ifetch < 3);)
    {
        s = natsSubscription_Fetch(&list, sub, 10, 5000, &jerr);
        if (s == NATS_OK)
            ifetch += (int64_t)list.Count;
        for (iack = 0; (s == NATS_OK) && (iack < list.Count); iack++)
        {
            s = natsMsg_Ack(list.Msgs[iack], &jsOpts);
            if (s == NATS_OK)
                printf("received message on %s\n", natsMsg_GetSubject(list.Msgs[iack]));
        }
        natsMsgList_Destroy(&list);
    }

    natsSubscription_Destroy(sub);
    jsCtx_Destroy(js);
    natsConnection_Destroy(nc);
    natsOptions_Destroy(opts);

    return 0;
}