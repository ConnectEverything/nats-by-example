// Install NuGet package `NATS.Net`
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server.
// Since connection is disposable at the end of our scope, we should flush
// our buffers and close the connection cleanly.
await using var nc = new NatsClient(url);

// Access JetStream for managing streams and consumers as well as for
// publishing and consuming messages to and from the stream.
var js = nc.CreateJetStreamContext();

var streamName = "EVENTS";

// ### Creating the stream
// Define the stream configuration, specifying `WorkQueuePolicy` for
// retention, and create the stream.
var stream = await js.CreateStreamAsync(new StreamConfig(streamName, subjects: ["events.>"])
{
    Retention = StreamConfigRetention.Workqueue,
});

// ### Queue messages
// Publish a few messages.
await js.PublishAsync("events.us.page_loaded", "event-data");
await js.PublishAsync("events.us.mouse_clicked", "event-data");
await js.PublishAsync("events.us.input_focused", "event-data");
Console.WriteLine("published 3 messages");

// Checking the stream info, we see three messages have been queued.
Console.WriteLine("# Stream info without any consumers");
await PrintStreamStateAsync(stream);

// ### Adding a consumer
// Now let's add a consumer and publish a few more messages.
// [pull](/examples/jetstream/pull-consumer/csharp)
var consumer = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-1"));

// Fetch and ack the queued messages.
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 3 }))
{
    await msg.AckAsync();
    /* await msg.AckAsync(new AckOpts { DoubleAck = true }); */
}

// Checking the stream info again, we will notice no messages
// are available.
Console.WriteLine("# Stream info with one consumer");
await PrintStreamStateAsync(stream);

// ### Exclusive non-filtered consumer
// As noted in the description above, work-queue streams can only have
// at most one consumer with interest on a subject at any given time.
// Since the pull consumer above is not filtered, if we try to create
// another one, it will fail.
Console.WriteLine("# Create an overlapping consumer");
try
{
    await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-2"));
}
catch (NatsJSApiException e)
{
    Console.WriteLine($"Error: {e.Error}");
}

// However, if we delete the first one, we can then add the new one.
await stream.DeleteConsumerAsync("processor-1");
await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-2"));
Console.WriteLine("Created the new consumer");

await stream.DeleteConsumerAsync("processor-2");

// ### Multiple filtered consumers
// To create multiple consumers, a subject filter needs to be applied.
// For this example, we could scope each consumer to the geo that the
// event was published from, in this case `us` or `eu`.
Console.WriteLine("# Create non-overlapping consumers");

var consumer1 = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-us") { FilterSubject = "events.us.>" });
var consumer2 = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-eu") { FilterSubject = "events.eu.>" });

await js.PublishAsync("events.eu.mouse_clicked", "event-data");
await js.PublishAsync("events.us.page_loaded", "event-data");
await js.PublishAsync("events.us.input_focused", "event-data");
await js.PublishAsync("events.eu.page_loaded", "event-data");
Console.WriteLine("Published 4 messages");

await foreach (var msg in consumer1.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    Console.WriteLine($"us sub got: {msg.Subject}");
    await msg.AckAsync();
}

await foreach (var msg in consumer2.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    Console.WriteLine($"eu sub got: {msg.Subject}");
    await msg.AckAsync();
}

// That's it!
Console.WriteLine("Bye!");

async Task PrintStreamStateAsync(INatsJSStream jsStream)
{
    await jsStream.RefreshAsync();
    var state = jsStream.Info.State;
    Console.WriteLine(
        $"Stream has messages:{state.Messages}" +
        $" first:{state.FirstSeq}" +
        $" last:{state.LastSeq}" +
        $" consumer_count:{state.ConsumerCount}" +
        $" num_subjects:{state.NumSubjects}");
}
