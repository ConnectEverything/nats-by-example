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

// Create `JetStream Context`, which provides methods to create
// streams and consumers as well as convenience methods for publishing
// to streams and consuming messages from the streams.
var js = nc.CreateJetStreamContext();

// ### Creating the stream
// Define the stream configuration, specifying `InterestPolicy` for retention, and
// create the stream.
var config = new StreamConfig(name: "EVENTS", subjects: ["events.>"])
{
    Retention = StreamConfigRetention.Interest,
};

var stream = await js.CreateStreamAsync(config);

// To demonstrate the base case behavior of the stream without any consumers, we
// will publish a few messages to the stream.
await js.PublishAsync<object>(subject: "events.page_loaded", data: null);
await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);
var ack = await js.PublishAsync<object>(subject: "events.input_focused", data: null);
Console.WriteLine("Published 3 messages");

// We confirm that all three messages were published and the last message sequence
// is 3.
Console.WriteLine($"Last message seq: {ack.Seq}");

// Checking out the stream info, notice how zero messages are present in
// the stream, but the `last_seq` is 3 which matches the last ACKed
// publish-sequence above.
// Also notice that the `first_seq` is one greater
// that behaves as a sentinel value indicating the stream is empty.
// This sequence has not been assigned to a message yet, but can be interpreted
// as _no messages available_ in this context.
Console.WriteLine("# Stream info without any consumers");
await PrintStreamStateAsync(stream);

// ### Adding a consumer
// Now let's add a pull consumer and publish a few
// more messages. Also note that we are _only_ creating the consumer and
// have not yet started consuming the messages. This is only to point out
// that it is not _required_ to be actively consuming messages to show
// _interest_, but it is the presence of a consumer which the stream cares
// about to determine retention of messages. [pull](/examples/jetstream/pull-consumer/csharp)
var consumer = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-1")
{
    AckPolicy = ConsumerConfigAckPolicy.Explicit,
});

await js.PublishAsync<object>(subject: "events.page_loaded", data: null);
await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);

// If we inspect the stream info again, we will notice a few differences.
// It shows two messages (which we expect) and the first and last sequences
// corresponding to the two messages we just published. We also see that
// the `consumer_count` is now one.
Console.WriteLine("# Stream info with one consumer");
await PrintStreamStateAsync(stream);

await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    await msg.AckAsync(new AckOpts { DoubleAck = true });
}

// What do we expect in the stream?
// No messages and the `first_seq` have been set to
// the _next_ sequence number like in the base case.
// ☝️ As quick aside on that second ack, We are using `AckSync` here for this
// example to ensure the stream state has been synced up for this subsequent
// retrieval.
Console.WriteLine("# Stream info with one consumer and acked messages");
await PrintStreamStateAsync(stream);

// ### Two or more consumers
// Since each consumer represents a separate _view_ over a stream, we would expect
// that if messages were processed by one consumer, but not the other, the messages
// would be retained. This is indeed the case.
var consumer2 = await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-2")
{
    AckPolicy = ConsumerConfigAckPolicy.Explicit,
});

await js.PublishAsync<object>(subject: "events.page_loaded", data: null);
await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);

// Here we fetch 2 messages for `processor-2`. There are two observations to
// make here. First, the fetched messages are the latest two messages that
// were published just above and not any prior messages since these were
// already deleted from the stream. This should be clear now, but this
// reinforces that a _late_ consumer cannot retroactively show interest. The
// second point is that the stream info shows that the latest two messages
// are still present in the stream. This is also expected since the first
// consumer had not yet processed them.
var msgMetas = new List<NatsJSMsgMetadata>();
await foreach (var msg in consumer2.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    await msg.AckAsync(new AckOpts { DoubleAck = true });
    if (msg.Metadata is { } metadata)
    {
        msgMetas.Add(metadata);
    }
}

Console.WriteLine($"msg sequences {msgMetas[0].Sequence.Stream} and {msgMetas[1].Sequence.Stream}");

Console.WriteLine("# Stream info with two consumers, but only one set of acked messages");
await PrintStreamStateAsync(stream);

// Fetching and acknowledging from the first consumer subscription will result in the messages
// being deleted.
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 2 }))
{
    await msg.AckAsync(new AckOpts { DoubleAck = true });
}

Console.WriteLine("# Stream info with two consumers having both acked");
await PrintStreamStateAsync(stream);

// A final callout is that _interest_ respects the `FilterSubject` on a consumer.
// For example, if a consumer defines a filter only for `events.mouse_clicked` events
// then it won't be considered _interested_ in events such as `events.input_focused`.
await stream.CreateOrUpdateConsumerAsync(new ConsumerConfig("processor-3")
{
    AckPolicy = ConsumerConfigAckPolicy.Explicit,
    FilterSubject = "events.mouse_clicked",
});

await js.PublishAsync<object>(subject: "events.input_focused", data: null);

// Fetch and `Terminate` (also works) and ack from the first consumers that _do_ have interest.
await foreach (var msg in consumer.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1 }))
{
    await msg.AckTerminateAsync();
}

await foreach (var msg in consumer2.FetchAsync<string>(opts: new NatsJSFetchOpts { MaxMsgs = 1 }))
{
    await msg.AckAsync(new AckOpts { DoubleAck = true });
}

Console.WriteLine("# Stream info with three consumers with interest from two");
await PrintStreamStateAsync(stream);

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
