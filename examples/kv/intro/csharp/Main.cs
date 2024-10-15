// Install NuGet package `NATS.Net`
using NATS.Client.JetStream.Models;
using NATS.Client.KeyValueStore;
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server.
// Since connection is disposable at the end of our scope, we should flush
// our buffers and close the connection cleanly.
await using var nc = new NatsClient(url);
var kv = nc.CreateKeyValueStoreContext();

// ### Bucket basics
// A key-value (KV) bucket is created by specifying a bucket name.
var profiles = await kv.CreateStoreAsync(new NatsKVConfig("profiles"));

// As one would expect, the `KeyValue` interface provides the
// standard `Put` and `Get` methods. However, unlike most KV
// stores, a revision number of the entry is tracked.
await profiles.PutAsync("sue.color", "blue");
var entry =  await profiles.GetEntryAsync<string>("sue.color");
Console.WriteLine($"{entry.Key} @ {entry.Revision} ->{entry.Value}\n");

await profiles.PutAsync("sue.color", "green");
entry =  await profiles.GetEntryAsync<string>("sue.color");
Console.WriteLine($"{entry.Key} @ {entry.Revision} ->{entry.Value}\n");

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
    Console.WriteLine($"Expected error: {e.Message}");
}

await profiles.UpdateAsync("sue.color", "red", 2);
entry =  await profiles.GetEntryAsync<string>("sue.color");
Console.WriteLine($"{entry.Key} @ {entry.Revision} ->{entry.Value}\n");

// ### Stream abstraction
// Before moving on, it is important to understand that a KV bucket is
// light abstraction over a standard stream. This is by design since it
// enables some powerful features that we will observe in a minute.
//
// **How exactly is a KV bucket modeled as a stream?**
// When one is created, internally, a stream is created using the `KV_`
// prefix as convention. Appropriate stream configuration is used that
// are optimized for the KV access patterns, so you can ignore the
// details.
var js = nc.CreateJetStreamContext();
await foreach (var name in js.ListStreamNamesAsync())
{
    Console.WriteLine($"KV stream name: {name}");
}

// Since it is a normal stream, we can create a consumer and
// fetch messages.
// If we look at the subject, we will notice that the first token is a
// special reserved prefix, the second token is the bucket name, and
// the remaining suffix is the actually key.
// The bucket name is inherently
// a namespace for all keys, and thus there is no concern for conflict
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
        Console.WriteLine($"{msg.Subject} @ {metadata.Sequence.Stream} -> {msg.Data}");
    }
}

// Let's put a new value for this key and see what we get from the subscription.
await profiles.PutAsync("sue.color", "yellow");
{
    var next = await consumer.NextAsync<string>();
    if (next is { Metadata: { } metadata } msg)
    {
        Console.WriteLine($"{msg.Subject} @ {metadata.Sequence.Stream} -> {msg.Data}");
    }
}

// Unsurprisingly, we get the new updated value as a message.
// Since it's a KV interface, we should be able to delete a key as well.
// Does this result in a new message?
await profiles.DeleteAsync("sue.color");
{
    var next = await consumer.NextAsync<string>();
    if (next is { Metadata: { } metadata } msg)
    {
        Console.WriteLine($"{msg.Subject} @ {metadata.Sequence.Stream} -> {msg.Data}");

        // 🤔 That is useful to get a message that something happened to that key,
        // and that this is considered a new revision.
        // However, how do we know if the new value was set to be `nil` or the key
        // was deleted?
        // To differentiate, delete-based messages contain a header. Notice the `KV-Operation: DEL`
        // header.
        Console.WriteLine($"Headers: {msg.Headers}");
    }
}

// ### Watching for changes
// Although one could subscribe to the stream directly, it is more convenient
// to use a `KeyWatcher` which provides a deliberate API and types for tracking
// changes over time.
// Notice that we can use a wildcard which we will come back to.
var watcher = Task.Run(async () => {
    await foreach (var kve in profiles.WatchAsync<string>())
    {
        Console.WriteLine($"{kve.Key} @ {kve.Revision} -> {kve.Value} (op: {kve.Operation})");
        if (kve.Key == "sue.food")
            break;
    }
});

// Even though we deleted the key, of course we can put a new value.
await profiles.PutAsync("sue.color", "purple");

// To finish this short intro, since we know that keys are subjects under the covers, if we
// put another key, we can observe the change through the watcher. One other detail to call out
// is notice the revision for this *new* key is not `1`. It relies on the underlying stream's
// message sequence number to indicate the _revision_. The guarantee is that it is always
// monotonically increasing, but numbers will be shared across keys (like subjects) rather
// than sequence numbers relative to each key.
await profiles.PutAsync("sue.food", "pizza");

await watcher;

// That's it!
Console.WriteLine("Bye!");
