// Install NuGet package `NATS.Net`
using NATS.Client.ObjectStore;
using NATS.Client.ObjectStore.Models;
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server.
// Since connection is disposable at the end of our scope, we should flush
// our buffers and close the connection cleanly.
await using var nc = new NatsClient(url);
var obj = nc.CreateObjectStoreContext();

// ### Object store basics
// An object-store (OS) bucket is created by specifying a bucket name.
// Here we try to access a store called "configs", if it doesn't exist,
// the API will create it:
var store = await obj.CreateObjectStoreAsync("configs");

// You can get information on the object store by getting its info:
var status = await store.GetStatusAsync();
Console.WriteLine($"The object store has {status.Info.State.Bytes} bytes");

// 10MiB
const int bytes = 10_000_000;
var data = new byte[bytes];

// Let's add an entry to the object store
var info = await store.PutAsync(key: "a", data);
Console.WriteLine($"Added entry {info.Name} ({info.Size} bytes)- '{info.Description}'");

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
    Console.WriteLine($"Entry {info.Name} ({info.Size} bytes)- '{info.Description}'");
    count++;
}
Console.WriteLine($"The object store contains {count} entries");

// Now let's retrieve the item we added
var data1 = await store.GetBytesAsync("a");
Console.WriteLine($"Data has {data1.Length} bytes");

// You can watch an object store for changes:
var watcher = Task.Run(async () =>
{
    await foreach (var m in store.WatchAsync(new NatsObjWatchOpts{IncludeHistory = false}))
    {
        var op = m.Deleted ? "was deleted" : "was updated";
        Console.WriteLine($">>>>>>>> Watch: {m.Bucket} changed '{m.Name}' {op}");
    }
});

// To delete an entry:
await store.DeleteAsync("a");

// Because the client may be working with large assets, ObjectStore
// normally presents a "Stream" based API.
info = await store.PutAsync(new ObjectMetadata { Name = "b", Description = "set with a stream" }, new MemoryStream(data));
Console.WriteLine($"Added entry {info.Name} ({info.Size} bytes)- '{info.Description}'");

var ms = new MemoryStream();
info = await store.GetAsync("b", ms);
Console.WriteLine($"Got entry {info.Name} ({info.Size} bytes)- '{info.Description}'");

await obj.DeleteObjectStore("configs", CancellationToken.None);

// That's it!
Console.WriteLine("Bye!");
