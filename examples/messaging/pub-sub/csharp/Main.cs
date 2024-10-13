// Install NuGet package `NATS.Net`
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
await using var nc = new NatsClient(url);

// Subscribe to a subject and start waiting for messages in the background.
Console.WriteLine("Waiting for messages...");
var cts = new CancellationTokenSource();
var subscriptionTask = Task.Run(async () =>
{
    await foreach (var msg in nc.SubscribeAsync<Order>("orders.>", cancellationToken: cts.Token))
    {
        var order = msg.Data;
        Console.WriteLine($"Subscriber received {msg.Subject}: {order}");
    }

    Console.WriteLine("Unsubscribed");
});

// Wait a bit before publishing orders, so we know the subscriber is ready.
await Task.Delay(1000);

// Let's publish a few orders.
for (int i = 0; i < 5; i++)
{
    Console.WriteLine($"Publishing order {i}...");
    await nc.PublishAsync($"orders.new.{i}", new Order(OrderId: i));
    await Task.Delay(500);
}

// We can unsubscribe now all orders are published. Cancelling the subscription
// should complete the message loop and exit the background task cleanly.
await cts.CancelAsync();
await subscriptionTask;

// That's it! We saw how we can subscribe to a subject and publish messages that would
// be seen by the subscribers based on matching subjects.
Console.WriteLine("Bye!");

public record Order(int OrderId);
