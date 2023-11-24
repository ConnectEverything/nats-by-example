// Install NuGet packages `NATS.Net`, `System.Interactive.Async` and `Microsoft.Extensions.Logging.Console`.

using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using NATS.Client.Core;

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
    Name = "NATS-by-Example",
};
await using var nats = new NatsConnection(opts);

await nats.ConnectAsync();

using var cts = new CancellationTokenSource();

var s1 = nats.SubscribeAsync<int>("s1", cancellationToken: cts.Token);
var s2 = nats.SubscribeAsync<int>("s2", cancellationToken: cts.Token);
var s3 = nats.SubscribeAsync<int>("s3", cancellationToken: cts.Token);
var s4 = nats.SubscribeAsync<int>("s4", cancellationToken: cts.Token);

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
    await nats.PublishAsync("s1", i);
    await nats.PublishAsync("s2", i);
    await nats.PublishAsync("s3", i);
    await nats.PublishAsync("s4", i);
    await Task.Delay(100);
}

await subs;

// That's it!
logger.LogInformation("Bye!");
