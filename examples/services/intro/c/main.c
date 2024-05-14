#include <stdio.h>
#include <nats.h>

#define MODE_MIN 0
#define MODE_MAX 1
#define MODE_AVERAGE 2

static microError *handler(microRequest *req);

int main()
{
    natsStatus s = NATS_OK;
    microError *err = NULL;
    natsOptions *opts = NULL;
    natsConnection *nc = NULL;
    natsMsg *msg = NULL;
    microService *svc = NULL;
    microServiceInfo *info = NULL;
    microGroup *grp = NULL;

    // A service configuration. Service definitions are made up of the service
    // name (which can't have things like whitespace in it), a version, and a
    // description.
    microServiceConfig svcConfig = {
        .Name = "MinMax",
        .Version = "0.0.1",
        .Description = "Returns the min/max number in a request",
    };

    // Configurations for the 3 endpioints that will be added to the service.
    // They share the same `handler`, which uses the `State` field to determine
    // which value to calculate and return.
    microEndpointConfig minConfig = {
        .Name = "min",
        .Handler = handler,
        .State = (void *)MODE_MIN,
    };
    microEndpointConfig maxConfig = {
        .Name = "max",
        .Handler = handler,
        .State = (void *)MODE_MAX,
    };
    microEndpointConfig averageConfig = {
        .Name = "average",
        .Handler = handler,
        .State = (void *)MODE_AVERAGE,
    };

    // Use the env variable if running in the container, otherwise use the
    // default.
    if ((s = natsOptions_Create(&opts)) != NATS_OK)
        goto _cleanup;

    const char *url = getenv("NATS_URL");
    if (url != NULL)
        s = natsOptions_SetURL(opts, url);

    // Create an unauthenticated connection to NATS.
    if ((s = natsConnection_Connect(&nc, opts)) != NATS_OK)
        goto _cleanup;

    // Add a new service, with the above config. Even with no running endpoints,
    // this service is discoverable via the micro protocol and by service
    // discovery tools like `nats micro`. All of the default background handlers
    // for discovery, PING, and stats are started at this point.
    if ((err = micro_AddService(&svc, nc, &svcConfig)) != NULL)
        goto _cleanup;

    // Each time we create a service, it will be given a new unique identifier.
    // If multiple copies of the `MinMax` service are running across a NATS
    // subject space, then tools like `nats micro` will consider them like
    // unique instances of the one service and the endpoint subscriptions are
    // queue subscribed, so requests will only be sent to one endpoint
    // _instance_ at a time.
    if ((err = microService_GetInfo(&info, svc)) != NULL)
        goto _cleanup;
    printf("Created service: Name:'%s', ID:'%s'\n", info->Name, info->Id);

    // Groups serve as namespaces and are used as a subject prefix when endpoints
    // don't supply fixed subjects. In this case, all endpoints will be listening
    // on a subject that starts with `func.`.
    if ((err = microService_AddGroup(&grp, svc, "func")) != NULL)
        goto _cleanup;

    // Add three endpoints to the service: `min`, `max`, and `average`. Each
    // endpoint represents a subscription that can process a `\n`-separated list
    // of integer numbers. The supplied `handler` uses the endpoint `State` to
    // determine which value to calculate and return. will respond to
    // `minmax.min` and `minmax.max`, respectively.
    if ((err = microGroup_AddEndpoint(grp, &minConfig)) != NULL)
        goto _cleanup;
    if ((err = microGroup_AddEndpoint(grp, &maxConfig)) != NULL)
        goto _cleanup;
    if ((err = microGroup_AddEndpoint(grp, &averageConfig)) != NULL)
        goto _cleanup;

    // Now we can use standard NATS requests to communicate with the service
    // endpoints. First, try the 3 endpoints themselves.
    if ((s = natsConnection_Request(&msg, nc, "func.min", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", 21, 1000)) != NATS_OK)
        goto _cleanup;
    printf("min response: %.*s\n", natsMsg_GetDataLength(msg), natsMsg_GetData(msg));
    natsMsg_Destroy(msg);
    msg = NULL;

    if ((s = natsConnection_Request(&msg, nc, "func.max", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", 21, 1000)) != NATS_OK)
        goto _cleanup;
    printf("max response: %.*s\n", natsMsg_GetDataLength(msg), natsMsg_GetData(msg));
    natsMsg_Destroy(msg);
    msg = NULL;

    if ((s = natsConnection_Request(&msg, nc, "func.average", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", 21, 1000)) != NATS_OK)
        goto _cleanup;
    printf("average response: %.*s\n", natsMsg_GetDataLength(msg), natsMsg_GetData(msg));
    natsMsg_Destroy(msg);
    msg = NULL;

    // Use the system subjects to request stats and info about the service.
    if ((s = natsConnection_Request(&msg, nc, "$SRV.STATS.MinMax", NULL, 0, 1000)) != NATS_OK)
        goto _cleanup;
    printf("STATS response: %.*s\n", natsMsg_GetDataLength(msg), natsMsg_GetData(msg));
    natsMsg_Destroy(msg);
    msg = NULL;

    if ((s = natsConnection_Request(&msg, nc, "$SRV.INFO.MinMax", NULL, 0, 1000)) != NATS_OK)
        goto _cleanup;
    printf("INFO response: %.*s\n", natsMsg_GetDataLength(msg), natsMsg_GetData(msg));
    natsMsg_Destroy(msg);
    msg = NULL;

    // Cleanup.
_cleanup:
    microService_Destroy(svc);
    natsConnection_Drain(nc);
    natsOptions_Destroy(opts);
    natsConnection_Destroy(nc);

    if (s != NATS_OK)
    {
        printf("Error: %u - %s\n", s, natsStatus_GetText(s));
        nats_PrintLastErrorStack(stderr);
        return 1;
    }
    if (err != NULL)
    {
        char errorbuf[2048];
        printf("Error: %s\n", microError_String(err, errorbuf, sizeof(errorbuf)));
        microError_Destroy(err);
        return 1;
    }
    return 0;
}

static microError *handler(microRequest *req)
{
    const char *data = microRequest_GetData(req);
    int len = microRequest_GetDataLength(req);
    int n = 0;
    char buf[1024];
    int min = INT32_MAX;
    int max = INT32_MIN;
    double average = 0.0;
    int line = 0;

    for (int i = 0; i < len; i++)
    {
        if (n >= sizeof(buf))
            return microRequest_RespondError(req, micro_Errorf("line %d too long, exceeds %d characters", line, sizeof(buf)));

        if ((i < (len - 1)) && (data[i] != '\n'))
        {
            buf[n++] = data[i];
            continue;
        }

        line++;
        buf[n++] = data[i];
        buf[n] = '\0';
        n = 0;
        int v = atoi(buf);
        average += (double)v;

        if (v < min)
            min = v;
        if (v > max)
            max = v;
    }
    average /= line;

    intptr_t mode = microRequest_GetEndpointState(req);
    switch (mode)
    {
    case MODE_MIN:
        n = snprintf(buf, sizeof(buf), "{\"min\":%d}", min);
        break;
    case MODE_MAX:
        n = snprintf(buf, sizeof(buf), "{\"max\":%d}", max);
        break;
    case MODE_AVERAGE:
        n = snprintf(buf, sizeof(buf), "{\"average\":%f}", average);
        break;
    default:
        return microRequest_RespondError(req, micro_Errorf("unknown mode: %d", mode));
    }

    return microRequest_Respond(req, buf, n + 1);
}
