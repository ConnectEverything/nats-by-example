// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.
using NATS.Client.Core;
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
var opts = new NatsOpts
{
    Url = url,
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

// Remove the stream first, so we have a clean starting point.
try
{
    await js.DeleteStreamAsync(stream);
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
Console.WriteLine("Consumer 1");
var consumer1 = await js.CreateOrUpdateConsumerAsync(stream, new ConsumerConfig(consumerName1));
Console.WriteLine("  Start");
Console.WriteLine($"    pending messages: {consumer1.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer1.Info.NumAckPending}");

var next = await consumer1.NextAsync<string>();

// refresh the consumer to update it's state
await consumer1.RefreshAsync();
Console.WriteLine("  After received but before ack");
Console.WriteLine($"    pending messages: {consumer1.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer1.Info.NumAckPending}");

if (next is { } msg1)
{
    await msg1.AckAsync();
}

await consumer1.RefreshAsync();
Console.WriteLine("  After ack");
Console.WriteLine($"    pending messages: {consumer1.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer1.Info.NumAckPending}");

// Consumer 2 Double Ack
var consumer2 = await js.CreateOrUpdateConsumerAsync(stream, new ConsumerConfig(consumerName2));
Console.WriteLine("Consumer 2");
Console.WriteLine("  Start");
Console.WriteLine($"    pending messages: {consumer1.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer1.Info.NumAckPending}");

next = await consumer2.NextAsync<string>();

await consumer2.RefreshAsync();
Console.WriteLine("  After received but before ack");
Console.WriteLine($"    pending messages: {consumer2.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer2.Info.NumAckPending}");

if (next is { } msg2)
{
    await msg2.AckAsync(new AckOpts { DoubleAck = true });
}

await consumer2.RefreshAsync();
Console.WriteLine("  After ack");
Console.WriteLine($"    pending messages: {consumer2.Info.NumPending}");
Console.WriteLine($"    messages with ack pending: {consumer2.Info.NumAckPending}");
