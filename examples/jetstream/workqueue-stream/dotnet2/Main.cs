// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.
using Microsoft.Extensions.Logging;
using NATS.Client.Core;
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;

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

// Access JetStream for managing streams and consumers as well as for
// publishing and consuming messages to and from the stream.
var js = new NatsJSContext(nats);

var streamName = "EVENTS";

// ### Creating the stream
// Define the stream configuration, specifying `WorkQueuePolicy` for
// retention, and create the stream.
var stream = await js.CreateStreamAsync(new StreamConfig(streamName, new[] { "events.>" })
{
    Retention = StreamConfigRetention.Workqueue,
});

// ### Queue messages
// Publish a few messages.
await js.PublishAsync("events.us.page_loaded", "event-data");
await js.PublishAsync("events.us.mouse_clicked", "event-data");
await js.PublishAsync("events.us.input_focused", "event-data");
logger.LogInformation("published 3 messages");

// Checking the stream info, we see three messages have been queued.
logger.LogInformation("# Stream info without any consumers");
await PrintStreamStateAsync(stream);

// ### Adding a consumer
// Now let's add a consumer and publish a few more messages.
// [pull](/examples/jetstream/pull-consumer/dotnet2)
var consumer = await stream.CreateConsumerAsync(new ConsumerConfig("processor-1"));

// Fetch and ack the queued messages.
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 3 }))
{
    await msg.AckAsync();
    /* await msg.AckAsync(new AckOpts { DoubleAck = true }); */
}

// Checking the stream info again, we will notice no messages
// are available.
logger.LogInformation("# Stream info with one consumer");
await PrintStreamStateAsync(stream);

// ### Exclusive non-filtered consumer
// As noted in the description above, work-queue streams can only have
// at most one consumer with interest on a subject at any given time.
// Since the pull consumer above is not filtered, if we try to create
// another one, it will fail.
logger.LogInformation("# Create an overlapping consumer");
try
{
    await stream.CreateConsumerAsync(new ConsumerConfig("processor-2"));
}
catch (NatsJSApiException e)
{
    logger.LogInformation("Error: {Message}", e.Error);
}

// However if we delete the first one, we can then add the new one.
await stream.DeleteConsumerAsync("processor-1");
await stream.CreateConsumerAsync(new ConsumerConfig("processor-2"));
logger.LogInformation("Created the new consumer");

await stream.DeleteConsumerAsync("processor-2");

// ### Multiple filtered consumers
// To create multiple consumers, a subject filter needs to be applied.
// For this example, we could scope each consumer to the geo that the
// event was published from, in this case `us` or `eu`.
logger.LogInformation("# Create non-overlapping consumers");

var consumer1 = await stream.CreateConsumerAsync(new ConsumerConfig("processor-us") { FilterSubject = "events.us.>" });
var consumer2 = await stream.CreateConsumerAsync(new ConsumerConfig("processor-eu") { FilterSubject = "events.eu.>" });

await js.PublishAsync("events.eu.mouse_clicked", "event-data");
await js.PublishAsync("events.us.page_loaded", "event-data");
await js.PublishAsync("events.us.input_focused", "event-data");
await js.PublishAsync("events.eu.page_loaded", "event-data");
logger.LogInformation("Published 4 messages");

await foreach (var msg in consumer1.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    logger.LogInformation("us sub got: {Subject}", msg.Subject);
    await msg.AckAsync();
}

await foreach (var msg in consumer2.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    logger.LogInformation("eu sub got: {Subject}", msg.Subject);
    await msg.AckAsync();
}

// That's it!
logger.LogInformation("Bye!");

async Task PrintStreamStateAsync(INatsJSStream jsStream)
{
    await jsStream.RefreshAsync();
    var state = jsStream.Info.State;
    logger.LogInformation(
        "Stream has messages:{Messages} first:{FirstSeq} last:{LastSeq} consumer_count:{ConsumerCount} num_subjects:{NumSubjects}",
        state.Messages,
        state.FirstSeq,
        state.LastSeq,
        state.ConsumerCount,
        state.NumSubjects);
}
