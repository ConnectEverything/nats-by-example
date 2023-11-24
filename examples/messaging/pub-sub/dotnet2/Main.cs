// Install NuGet packages `NATS.Net`, `NATS.Client.Serializers.Json` and `Microsoft.Extensions.Logging.Console`.
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

// Subscribe to a subject and start waiting for messages in the background.
await using var sub = await nats.SubscribeCoreAsync<Order>("orders.>");

logger.LogInformation("Waiting for messages...");
var task = Task.Run(async () =>
{
    await foreach (var msg in sub.Msgs.ReadAllAsync())
    {
        var order = msg.Data;
        logger.LogInformation("Subscriber received {Subject}: {Order}", msg.Subject, order);
    }

    logger.LogInformation("Unsubscribed");
});

// Let's publish a few orders.
for (int i = 0; i < 5; i++)
{
    logger.LogInformation("Publishing order {Index}...", i);
    await nats.PublishAsync($"orders.new.{i}", new Order(OrderId: i));
    await Task.Delay(500);
}

// We can unsubscribe now all orders are published. Unsubscribing or disposing the subscription
// should complete the message loop and exit the background task cleanly.
await sub.UnsubscribeAsync();
await task;

// That's it! We saw how we can subscribe to a subject and publish messages that would
// be seen by the subscribers based on matching subjects.
logger.LogInformation("Bye!");

public record Order(int OrderId);
