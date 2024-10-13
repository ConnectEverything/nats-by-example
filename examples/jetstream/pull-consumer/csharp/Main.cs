// Install NuGet package `NATS.Net`
using System.Diagnostics;
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
await using var nc = new NatsClient(url);

// Access JetStream for managing streams and consumers as well as for
// publishing and consuming messages to and from the stream.
var js = nc.CreateJetStreamContext();

var streamName = "EVENTS";

// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/csharp/).
var stream = await js.CreateStreamAsync(new StreamConfig(streamName, subjects: ["events.>"]));

// Publish a few messages for the example.
await js.PublishAsync(subject: "events.1", data: "event-data-1");
await js.PublishAsync(subject: "events.2", data: "event-data-2");
await js.PublishAsync(subject: "events.3", data: "event-data-3");

// Create the consumer bound to the previously created stream. If durable
// name is not supplied, consumer will be removed after InactiveThreshold
// (defaults to 5 seconds) is reached when not actively consuming messages.
// `Name` is optional, if not provided it will be auto-generated.
// For this example, let's use the consumer with no options, which will
// be ephemeral with auto-generated name.
var consumer = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig());

// Messages can be _consumed_  continuously in a loop using `Consume`
// method. `Consume` can be supplied with various options, but for this
// example we will use the default ones.`break` is used as part of this
// example to make sure to stop processing  after we process 3 messages (so
// that it does not interfere with other examples).
var count = 0;
await foreach (var msg in consumer.ConsumeAsync<string>())
{
    await msg.AckAsync();
    Console.WriteLine($"received msg on {msg.Subject} with data {msg.Data}");
    if (++count == 3)
        break;
}

// Publish more messages.
await js.PublishAsync(subject: "events.1", data: "event-data-1");
await js.PublishAsync(subject: "events.2", data: "event-data-2");
await js.PublishAsync(subject: "events.3", data: "event-data-3");

// We can _fetch_ messages in batches. The first argument being the
// batch size which is the _maximum_ number of messages that should
// be returned. For this first fetch, we ask for two and we will get
// those since they are in the stream.
var fetchCount = 0;
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    await msg.AckAsync();
    fetchCount++;
}

Console.WriteLine($"Got {fetchCount} messages");

// `Fetch` puts messages on the returned `Messages()` channel. This channel
// will only be closed when the requested number of messages have been
// received or the operation times out. If we do not want to wait for the
// rest of the messages and want to quickly return as many messages as there
// are available (up to provided batch size), we can use `FetchNoWait`
// instead.
// Here, because we have already received two messages, we will only get
// one more.
// NOTE: `FetchNoWait` usage is discouraged since it can cause unnecessary load
// if not used correctly e.g. in a loop without a backoff it will continuously
// try to get messages even if there is no new messages in the stream.
fetchCount = 0;
await foreach (var msg in ((NatsJSConsumer)consumer).FetchNoWaitAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 100 }))
{
    await msg.AckAsync();
    fetchCount++;
}
Console.WriteLine($"Got {fetchCount} messages");

// Finally, if we are at the end of the stream and we call fetch,
// the call will be blocked until the "max wait" time which is 30
// seconds by default, but this can be set explicitly as an option.
var fetchStopwatch = Stopwatch.StartNew();
fetchCount = 0;
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 100, Expires = TimeSpan.FromSeconds(1) }))
{
    await msg.AckAsync();
    fetchCount++;
}
Console.WriteLine($"Got {fetchCount} messages in {fetchStopwatch.Elapsed}");

// Durable consumers can be created by specifying the Durable name.
// Durable consumers are not removed automatically regardless of the
// InactiveThreshold. They can be removed by calling `DeleteConsumer`.
var durable = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor"));

// Consume and fetch work the same way for durable consumers.
await foreach (var msg in durable.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1 }))
{
    Console.WriteLine($"Received {msg.Subject} from durable consumer");
}

// While ephemeral consumers will be removed after InactiveThreshold, durable
// consumers have to be removed explicitly if no longer needed.
await stream.DeleteConsumerAsync("processor");

// Let's try to get the consumer to make sure it's gone.
try
{
    await stream.GetConsumerAsync("processor");
}
catch (NatsJSApiException e)
{
    if (e.Error.Code == 404)
    {
        Console.WriteLine("Consumer is gone");
    }
}

// That's it!
Console.WriteLine("Bye!");
