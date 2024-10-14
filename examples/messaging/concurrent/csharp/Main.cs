// Install NuGet package `NATS.Net`
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
await using var nc = new NatsClient(url);

using var cts = new CancellationTokenSource();

// Subscribe to a subject and start waiting for messages in the background and
// start processing messages in parallel.
var subscription = Task.Run(async () =>
{
    await Parallel.ForEachAsync(nc.SubscribeAsync<string>("greet", cancellationToken: cts.Token), (msg, _) =>
    {
        Console.WriteLine($"Received {msg.Data}");
        return ValueTask.CompletedTask;
    });
});

// Give some time for the subscription to start.
await Task.Delay(TimeSpan.FromSeconds(1));

for (int i = 0; i < 50; i++)
{
    await nc.PublishAsync("greet", $"hello {i}");
}

// Give some time for the subscription to receive all the messages.
await Task.Delay(TimeSpan.FromSeconds(1));

await cts.CancelAsync();

await subscription;

// That's it!
Console.WriteLine("Bye!");
