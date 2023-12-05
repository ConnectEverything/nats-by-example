// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.

using System;
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

using var cts = new CancellationTokenSource();

// Subscribe to a subject and start waiting for messages in the background and
// start processing messages in parallel.
var subscription = Task.Run(async () =>
{
    await Parallel.ForEachAsync(nats.SubscribeAsync<string>("greet", cancellationToken: cts.Token), async (msg, _) =>
    {
        Console.WriteLine($"Received {msg.Data}");
    });
});

// Give some time for the subscription to start.
await Task.Delay(TimeSpan.FromSeconds(1));

for (int i = 0; i < 50; i++)
{
    await nats.PublishAsync("greet", $"hello {i}");
}

// Give some time for the subscription to receive all the messages.
await Task.Delay(TimeSpan.FromSeconds(1));

await cts.CancelAsync();

await subscription;

// That's it!
logger.LogInformation("Bye!");
