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
var js = new NatsJSContext(nats);
var kv = new NatsKVContext(js);

// ### Bucket basics
// A key-value (KV) bucket is created by specifying a bucket name.
var profiles = await kv.CreateStoreAsync(new NatsKVConfig("profiles"));

// As one would expect, the `KeyValue` interface provides the
// standard `Put` and `Get` methods. However, unlike most KV
// stores, a revision number of the entry is tracked.
await profiles.PutAsync("sue.color", "blue");
var entry =  await profiles.GetEntryAsync<string>("sue.color");
logger.LogInformation("{Key} @ {Revision} ->{Value}\n", entry.Key, entry.Revision, entry.Value);

await profiles.PutAsync("sue.color", "green");
entry =  await profiles.GetEntryAsync<string>("sue.color");
logger.LogInformation("{Key} @ {Revision} ->{Value}\n", entry.Key, entry.Revision, entry.Value);

// A revision number is useful when you need to enforce [optimistic
// concurrency control][occ] on a specific key-value entry. In short,
// if there are multiple actors attempting to put a new value for a
// key concurrently, we want to prevent the "last writer wins" behavior
// which is non-deterministic. To guard against this, we can use the
// `kv.Update` method and specify the expected revision. Only if this
// matches on the server, will the value be updated.
// [occ]: https://en.wikipedia.org/wiki/Optimistic_concurrency_control
try
{
    await profiles.UpdateAsync("sue.color", "red", 1);
}
catch (NatsKVWrongLastRevisionException e)
{
    logger.LogInformation("Expected error: {Error}", e.Message);
}

await profiles.UpdateAsync("sue.color", "red", 2);
entry =  await profiles.GetEntryAsync<string>("sue.color");
logger.LogInformation("{Key} @ {Revision} ->{Value}\n", entry.Key, entry.Revision, entry.Value);

// ### Stream abstraction
// Before moving on, it is important to understand that a KV bucket is
// light abstraction over a standard stream. This is by design since it
// enables some powerful features which we will observe in a minute.
//
// **How exactly is a KV bucket modeled as a stream?**
// When one is created, internally, a stream is created using the `KV_`
// prefix as convention. Appropriate stream configuration are used that
// are optimized for the KV access patterns, so you can ignore the
// details.
await foreach (var name in js.ListStreamNamesAsync())
{
    logger.LogInformation("KV stream name: {Name}", name);
}

// Since it is a normal stream, we can create a consumer and
// fetch messages.
// If we look at the subject, we will notice that first token is a
// special reserved prefix, the second token is the bucket name, and
// remaining suffix is the actually key. The bucket name is inherently
// a namespace for all keys and thus there is no concern for conflict
// across buckets. This is different from what we need to do for a stream
// which is to bind a set of _public_ subjects to a stream.
var consumer = await js.CreateConsumerAsync("KV_profiles", new ConsumerConfig
{
    AckPolicy = ConsumerConfigAckPolicy.None,
});

{
    var next = await consumer.NextAsync<string>();
    if (next is { Metadata: { } metadata } msg)
    {
        logger.LogInformation("{Subject} @ {Sequence} -> {Data}", msg.Subject, metadata.Sequence.Stream, msg.Data);
    }
}

// Let's put a new value for this key and see what we get from the subscription.
await profiles.PutAsync("sue.color", "yellow");
{
    var next = await consumer.NextAsync<string>();
    if (next is { Metadata: { } metadata } msg)
    {
        logger.LogInformation("{Subject} @ {Sequence} -> {Data}", msg.Subject, metadata.Sequence.Stream, msg.Data);
    }
}

// Unsurprisingly, we get the new updated value as a message.
// Since it's KV interface, we should be able to delete a key as well.
// Does this result in a new message?
await profiles.DeleteAsync("sue.color");
{
    var next = await consumer.NextAsync<string>();
    if (next is { Metadata: { } metadata } msg)
    {
        logger.LogInformation("{Subject} @ {Sequence} -> {Data}", msg.Subject, metadata.Sequence.Stream, msg.Data);

        // 🤔 That is useful to get a message that something happened to that key,
        // and that this is considered a new revision.
        // However, how do we know if the new value was set to be `nil` or the key
        // was deleted?
        // To differentiate, delete-based messages contain a header. Notice the `KV-Operation: DEL`
        // header.
        logger.LogInformation("Headers: {Headers}", msg.Headers);
    }
}

// ### Watching for changes
// Although one could subscribe to the stream directly, it is more convenient
// to use a `KeyWatcher` which provides a deliberate API and types for tracking
// changes over time. Notice that we can use a wildcard which we will come back to..
var watcher = Task.Run(async () => {
    await foreach (var kve in profiles.WatchAsync<string>())
    {
        logger.LogInformation("{Key} @ {Revision} -> {Value} (op: {Op})", kve.Key, kve.Revision, kve.Value, kve.Operation);
        if (kve.Key == "sue.food")
            break;
    }
});

// Even though we deleted the key, of course we can put a new value.
await profiles.PutAsync("sue.color", "purple");

// To finish this short intro, since we know that keys are subjects under the covers, if we
// put another key, we can observe the change through the watcher. One other detail to call out
// is notice the revision for this *new* key is not `1`. It relies on the underlying stream's
// message sequence number to indicate the _revision_. The guarantee being that it is always
// monotonically increasing, but numbers will be shared across keys (like subjects) rather
// than sequence numbers relative to each key.
await profiles.PutAsync("sue.food", "pizza");

await watcher;

// That's it!
logger.LogInformation("Bye!");
