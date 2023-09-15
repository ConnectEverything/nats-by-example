// Install `NATS.Client.Core` from NuGet.
using NATS.Client.Core;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "127.0.0.1:4222";
Console.WriteLine($"Connecting to {url}...");

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
var opts = NatsOpts.Default with { Url = url };
await using var nats = new NatsConnection(opts);

// Subscribe to a subject and start waiting for messages in the background.
await using var sub = await nats.SubscribeAsync<Order>("orders.>");

Console.WriteLine("[SUB] waiting for messages...");
var task = Task.Run(async () =>
{
    await foreach (var msg in sub.Msgs.ReadAllAsync())
    {
        var order = msg.Data;
        Console.WriteLine($"[SUB] received {msg.Subject}: {order}");
    }

    Console.WriteLine($"[SUB] unsubscribed");
});

// Let's publish a few orders.
for (int i = 0; i < 5; i++)
{
    Console.WriteLine($"[PUB] publishing order {i}...");
    await nats.PublishAsync($"orders.new.{i}", new Order(OrderId: i));
    await Task.Delay(1_000);
}

// We can unsubscribe now all orders are published. Unsubscribing or disposing the subscription
// should complete the message loop and exit the background task cleanly.
await sub.UnsubscribeAsync();
await task;

// That's it! We saw how we can subscribe to a subject and publish messages that would be seen by the subscribers
// based on matching subjects.
Console.WriteLine("Bye!");

public record Order(int OrderId);
