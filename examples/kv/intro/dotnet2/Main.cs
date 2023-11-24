// Install NuGet packages `NATS.Net`, `NATS.Client.Serializers.Json` and `Microsoft.Extensions.Logging.Console`.

using System;
using Microsoft.Extensions.Logging;
using NATS.Client.Core;
using NATS.Client.Serializers.Json;

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


// That's it!
logger.LogInformation("Bye!");
