// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.

using System.Diagnostics;
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

// Declare a simple [limits-based stream](/examples/jetstream/limits-stream/dotnet2/).
var stream = await js.CreateStreamAsync(new StreamConfig(streamName, new[] { "events.>" }));

// Define a basic pull consumer without any limits and a short ack wait
// time for the purpose of this example. These default options will be
// reused when we update the consumer to show-case various limits.
// If you haven't seen the first [pull consumer][1] example yet, check
// that out first!
// [1]: /examples/jetstream/pull-consumer/dotnet2/
var consumerName = "processor";
var ackWait = TimeSpan.FromSeconds(10);
var ackPolicy = ConsumerConfigAckPolicy.Explicit;
var maxWaiting = 1;

// One quick note. This example show cases how consumer configuration
// can be changed on-demand. This one exception is `MaxWaiting` which
// cannot be updated on a consumer as of now. This must be set up front
// when the consumer is created.
var consumer = await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
});

// ### Max in-flight messages
// The first limit to explore is the max in-flight messages. This
// will limit how many un-acked in-flight messages there are across
// all subscriptions bound to this consumer.
// We can update the consumer config on-the-fly with the
// `MaxAckPending` setting.
logger.LogInformation("--- max in-flight messages (n=1) ---");

await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
	MaxAckPending = 1,
});

// Let's publish a couple events for this section.
await js.PublishAsync(subject: "events.1", data: "event-data-1");
await js.PublishAsync(subject: "events.2", data: "event-data-2");

// We can request a larger batch size, but we will only get one
// back since only one can be un-acked at any given time. This
// essentially forces serial processing messages for a pull consumer.
var received = new List<NatsJSMsg<string>>();
await foreach (NatsJSMsg<string> msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 3, Expires = TimeSpan.FromSeconds(3) }))
{
	received.Add(msg);
}
logger.LogInformation("Requested 3, got {Count}", received.Count);


// This limit becomes more apparent with the second fetch which would
// timeout without any messages since we haven't acked the previous one yet.
var received2 = new List<NatsJSMsg<string>>();
await foreach (NatsJSMsg<string> msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1, Expires = TimeSpan.FromSeconds(1) }))
{
	received2.Add(msg);
}
logger.LogInformation("Requested 1, got {Count}", received2.Count);

// Let's ack it and then try another fetch.
await received[0].AckAsync();

// It works this time!
await foreach (NatsJSMsg<string> msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1 }))
{
	received2.Add(msg);
	await msg.AckAsync();
}
logger.LogInformation("Requested 1, got {Count}", received2.Count);

// ### Max fetch batch size
// This one limits the max batch size any one fetch can receive. This
// can be used to keep the fetches to a reasonable size.
logger.LogInformation("--- max fetch batch size (n=2) ---");

consumer = await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
	MaxBatch = 2,
});

// Publish a couple events for this section...
await js.PublishAsync(subject: "events.1", data: "hello");
await js.PublishAsync(subject: "events.2", data: "world");


// If a batch size is larger than the limit, it is considered an error.
// Because Fetch is non-blocking, we need to wait for the operation to
// complete before checking the error.
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 10, Expires = TimeSpan.FromSeconds(1) }))
{
}

// Using the max batch size (or less) will, of course, work.
var fetchCount = 0;
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
	logger.LogInformation("Received {Data}", msg.Data);
	await msg.AckAsync();
	fetchCount++;
}
logger.LogInformation("Requested 2, got {Count}", fetchCount);

// ### Max waiting requests
// The next limit defines the maximum number of fetch requests
// that are all waiting in parallel to receive messages. This
// prevents building up too many requests that the server will
// have to distribute to for a given consumer.
logger.LogInformation("--- max waiting requests (n=1) ---");

// Since `MaxWaiting` was already set to 1 when the consumer
// was created, this is a no-op.
await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
});

// Publish lots of events to trigger 409 Exceeded MaxWaiting.
for (int i = 0; i < 1000; i++)
{
	await js.PublishAsync(subject: "events.x", data: "event-data");
}

var cts = new CancellationTokenSource(TimeSpan.FromSeconds(3));
await foreach (var msg in consumer.ConsumeAsync<string>(opts: new NatsJSConsumeOpts { MaxMsgs = 100 }, cancellationToken: cts.Token))
{
}

await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1000 }))
{
	await msg.AckAsync();
}

// ### Max fetch timeout
// Normally each fetch call can specify it's own max wait timeout, i.e.
// how long the client wants to wait to receive at least one message.
// It may be desirable to limit defined on the consumer to prevent
// requests waiting too long for messages.
logger.LogInformation("--- max fetch timeout (d=1s) ---");

await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
	MaxExpires = (long)TimeSpan.FromSeconds(1).TotalNanoseconds,
});

// Using a max wait equal or less than `MaxRequestExpires` not return an
// error and return expected number of messages (zero in that case, since
// there are no more).
var fetchStopwatch = Stopwatch.StartNew();
fetchCount = 0;
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 10, Expires = TimeSpan.FromSeconds(1) }))
{
	fetchCount++;
}
logger.LogInformation("Got {Count} messages in {Elapsed}", fetchCount, fetchStopwatch.Elapsed);

// However, trying to use a longer timeout you'd get a warning `409 Exceeded MaxRequestExpires of 1s`
fetchStopwatch.Restart();
fetchCount = 0;
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 10, Expires = TimeSpan.FromSeconds(5) }))
{
	fetchCount++;
}
logger.LogInformation("Got {Count} messages in {Elapsed}", fetchCount, fetchStopwatch.Elapsed);


// ### Max total bytes per fetch
//
logger.LogInformation("--- max total bytes per fetch (n=4) ---");
/*	fmt.Println("\n--- max total bytes per fetch (n=4) ---")

	stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:               consumerName,
		AckPolicy:          ackPolicy,
		AckWait:            ackWait,
		MaxWaiting:         maxWaiting,
		MaxRequestMaxBytes: 3,
	})

	js.Publish(ctx, "events.3", []byte("hola"))
	js.Publish(ctx, "events.4", []byte("again"))

	msgs, _ = cons.FetchBytes(4)
	for range msgs.Messages() {
	}
	fmt.Printf("%s\n", msgs.Error())
*/
await stream.CreateConsumerAsync(new ConsumerConfig(consumerName)
{
	AckPolicy = ackPolicy,
	AckWait = (long)ackWait.TotalNanoseconds,
	MaxWaiting = maxWaiting,
	MaxBytes = 3,
});

await js.PublishAsync(subject: "events.1", data: "hi");
await js.PublishAsync(subject: "events.2", data: "again");

await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxBytes = 4, Expires = TimeSpan.FromSeconds(1) }))
{
}

// That's it!
logger.LogInformation("Bye!");
