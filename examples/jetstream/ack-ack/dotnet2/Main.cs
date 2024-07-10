// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Configuration;
using NATS.Client.Core;
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;
using NATS.Client.KeyValueStore;

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

// Get a JetStream Context
var js = new NatsJSContext(nats);

// ### Stream Setup
var stream = "verifyAckStream";
var subject = "verifyAckSubject";
var consumerName1 = "consumer1";
var consumerName2 = "consumer2";

// Remove the stream first!, so we have a clean starting point.
try
{
    await js.DeleteConsumerAsync("MY_STREAM", "consumer_non_existent");
}
catch (NatsJSApiException e) when (e is { Error.Code: 404 })
{
}

// Create the stream
var streamConfig = new StreamConfig(stream, [subject])
{
    Storage = StreamConfigStorage.Memory,
};
await js.CreateStreamAsync(streamConfig);

// Publish a couple messages, so we can look at the state
await js.PublishAsync(subject, "A");
await js.PublishAsync(subject, "B");

// Consume a message with 2 different consumers
// The first consumer will Ack without confirmation
// The second consumer will AckSync which confirms that ack was handled.

// Consumer 1, regular ack
var consumer1 = await js.CreateOrUpdateConsumerAsync(stream, new ConsumerConfig(consumerName1));
logger.LogInformation(
    "Consumer 1, Start # pending messages: {}, messages with ack pending: {}",
    consumer1.Info.NumPending,
    consumer1.Info.NumAckPending);

var next = await consumer1.NextAsync<string>();

// refresh the consumer to update it's state
await consumer1.RefreshAsync();
logger.LogInformation(
    "Consumer 1, After received but before ack # pending messages: {}, messages with ack pending: {}",
    consumer1.Info.NumPending,
    consumer1.Info.NumAckPending);

if (next is { } msg1)
{
    await msg1.AckAsync();
}

// refresh the consumer to update it's state
await consumer1.RefreshAsync();
logger.LogInformation(
    "Consumer 1, After ack # pending messages: {}, messages with ack pending: {}",
    consumer1.Info.NumPending,
    consumer1.Info.NumAckPending);

var consumer2 = await js.CreateOrUpdateConsumerAsync(stream, new ConsumerConfig(consumerName2));

logger.LogInformation(
    "Consumer 2, Start # pending messages: {}, messages with ack pending: {}",
    consumer2.Info.NumPending,
    consumer2.Info.NumAckPending);

next = await consumer2.NextAsync<string>();

// refresh the consumer to update it's state
await consumer2.RefreshAsync();
logger.LogInformation(
    "Consumer 2, After received but before ack # pending messages: {}, messages with ack pending: {}",
    consumer2.Info.NumPending,
    consumer2.Info.NumAckPending);

if (next is { } msg2)
{
    await msg2.AckAsync(new AckOpts { DoubleAck = true });
}

// refresh the consumer to update it's state
await consumer2.RefreshAsync();
logger.LogInformation(
    "Consumer 2, After ack # pending messages: {}, messages with ack pending: {}",
    consumer2.Info.NumPending,
    consumer2.Info.NumAckPending);

// That's it!
logger.LogInformation("Bye!");
