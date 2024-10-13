// Install NuGet packages `NATS.Net`, `NATS.Client.Serializers.Json` and `Microsoft.Extensions.Logging.Console`.

using System;
using Microsoft.Extensions.Logging;
using NATS.Client.Core;
using NATS.Client.Serializers.Json;
using NATS.Client.Services;

using var loggerFactory = LoggerFactory.Create(builder => builder.AddConsole());
var logger = loggerFactory.CreateLogger("NATS-by-Example");

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
var opts = new NatsOpts
{
    Url = url,
    LoggerFactory = loggerFactory,
    SerializerRegistry = NatsJsonSerializerRegistry.Default,
    Name = "NATS-by-Example",
};
await using var nats = new NatsConnection(opts);
var svc = new NatsSvcContext(nats);

// ### Defining a Service
//
// This will create a service definition. Service definitions are made up of
// the service name (which can't have things like whitespace in it), a version,
// and a description. Even with no running endpoints, this service is discoverable
// via the micro protocol and by service discovery tools like `nats micro`.
// All of the default background handlers for discovery, PING, and stats are
// started at this point.
var service = await svc.AddServiceAsync(new NatsSvcConfig("minmax", "0.0.1")
{
    Description = "Returns the min/max number in a request"
});

// Each time we create a service, it will be given a new unique identifier. If multiple
// copies of the `minmax` service are running across a NATS subject space, then tools
// like `nats micro` will consider them like unique instances of the one service and the
// endpoint subscriptions are queue subscribed, so requests will only be sent to one
// endpoint _instance_ at a time.
// TODO: service.Info

// ### Adding endpoints
//
// Groups serve as namespaces and are used as a subject prefix when endpoints
// don't supply fixed subjects. In this case, all endpoints will be listening
// on a subject that starts with `minmax.`
var root = await service.AddGroupAsync("minmax");

// Adds two endpoints to the service, one for the `min` operation and one for
// the `max` operation. Each endpoint represents a subscription. The supplied handlers
// will respond to `minmax.min` and `minmax.max`, respectively.
await root.AddEndpointAsync(HandleMin, "min", serializer: NatsJsonSerializer<int[]>.Default);
await root.AddEndpointAsync(HandleMax, "max", serializer: NatsJsonSerializer<int[]>.Default);

// Make a request of the `min` endpoint of the `minmax` service, within the `minmax` group.
// Note that there's nothing special about this request, it's just a regular NATS
// request.
var min = await nats.RequestAsync<int[], int>("minmax.min", new[]
{
    -1, 2, 100, -2000
}, requestSerializer: NatsJsonSerializer<int[]>.Default);
logger.LogInformation("Requested min value, got {Min}", min.Data);

// Make a request of the `max` endpoint of the `minmax` service, within the `minmax` group.
var max = await nats.RequestAsync<int[], int>("minmax.max", new[]
{
    -1, 2, 100, -2000
}, requestSerializer: NatsJsonSerializer<int[]>.Default);
logger.LogInformation("Requested max value, got {Max}", max.Data);

// The statistics being managed by micro should now reflect the call made
// to each endpoint, and we didn't have to write any code to manage that.
// TODO: service.Stats

// That's it!
logger.LogInformation("Bye!");

ValueTask HandleMin(NatsSvcMsg<int[]> msg)
{
    var min = msg.Data.Min();
    return msg.ReplyAsync(min);
}

ValueTask HandleMax(NatsSvcMsg<int[]> msg)
{
    var min = msg.Data.Max();
    return msg.ReplyAsync(min);
}