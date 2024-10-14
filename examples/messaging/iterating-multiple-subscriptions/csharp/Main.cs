// Install NuGet packages `NATS.Net` and `System.Interactive.Async`
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
await using var nc = new NatsClient(url);

await nc.ConnectAsync();

using var cts = new CancellationTokenSource();

var s1 = nc.SubscribeAsync<int>("s1", cancellationToken: cts.Token);
var s2 = nc.SubscribeAsync<int>("s2", cancellationToken: cts.Token);
var s3 = nc.SubscribeAsync<int>("s3", cancellationToken: cts.Token);
var s4 = nc.SubscribeAsync<int>("s4", cancellationToken: cts.Token);

const int total = 80;

var subs = Task.Run(async () =>
{
    var count = 0;
    await foreach (var msg in AsyncEnumerableEx.Merge(s1, s2, s3, s4))
    {
        Console.WriteLine($"Received {msg.Subject}: {msg.Data}");
        
        if (++count == total)
            await cts.CancelAsync();
    }
});

await Task.Delay(1000);

for (int i = 0; i < total / 4; i++)
{
    await nc.PublishAsync("s1", i);
    await nc.PublishAsync("s2", i);
    await nc.PublishAsync("s3", i);
    await nc.PublishAsync("s4", i);
    await Task.Delay(100);
}

await subs;

// That's it!
Console.WriteLine("Bye!");
