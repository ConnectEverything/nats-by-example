// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.
using Microsoft.Extensions.Logging;
using NATS.Client.Core;
using NATS.Client.JetStream;
using NATS.Client.ObjectStore;
using NATS.Client.ObjectStore.Models;

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
var obj = new NatsObjContext(js);

// ### Object store basics
// An object-store (OS) bucket is created by specifying a bucket name.
// Here we try to access a store called "configs", if it doesn't exist
// the API will create it:
var store = await obj.CreateObjectStore("configs");

// You can get information on the object store by getting its info:
var status = await store.GetStatusAsync();
logger.LogInformation("The object store has {Size} bytes", status.Info.State.Bytes);

// 10MiB
const int bytes = 10_000_000;
var data = new byte[bytes];

// Let's add an entry to the object store
var info = await store.PutAsync(key: "a", data);
logger.LogInformation("Added entry {Name} ({Size} bytes)- '{Description}'", info.Name, info.Size, info.Description);

// Entries in an object store are made from a "metadata" that describes the object
// And the payload. This allows you to store information about the significance of the
// entry separate from the raw data.
// You can update the metadata directly
await store.UpdateMetaAsync("a", new ObjectMetadata { Name = "a", Description = "still large data" });

// we expect this store to only contain one entry
// You can list its contents:
var count = 0;
await foreach (var entry in store.ListAsync())
{
    logger.LogInformation("Entry {Name} ({Size} bytes)- '{Description}'", info.Name, info.Size, info.Description);
    count++;
}
logger.LogInformation("The object store contains {Count} entries", count);

// Now lets retrieve the item we added
var data1 = await store.GetBytesAsync("a");
logger.LogInformation("Data has {Size} bytes", data1.Length);

// You can watch an object store for changes:
var watcher = Task.Run(async () =>
{
    await foreach (var m in store.WatchAsync(new NatsObjWatchOpts{IncludeHistory = false}))
    {
        logger.LogInformation(">>>>>>>> Watch: {Bucket} changed '{Name}' {Op}", m.Bucket, m.Name, m.Deleted ? "was deleted" : "was updated");
    }
});

// To delete an entry:
await store.DeleteAsync("a");

// Because the client may be working with large assets, ObjectStore
// normally presents a "Stream" based API.
info = await store.PutAsync(new ObjectMetadata { Name = "b", Description = "set with a stream" }, new MemoryStream(data));
logger.LogInformation("Added entry {Name} ({Size} bytes)- '{Description}'", info.Name, info.Size, info.Description);

var ms = new MemoryStream();
info = await store.GetAsync("b", ms);
logger.LogInformation("Got entry {Name} ({Size} bytes)- '{Description}'", info.Name, info.Size, info.Description);

await obj.DeleteObjectStore("configs", CancellationToken.None);

// That's it!
logger.LogInformation("Bye!");
