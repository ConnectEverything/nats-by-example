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

// We will declare the initial stream configuration by specifying
// the name and subjects. Stream names are commonly uppercase to
// visually differentiate them from subjects, but this is not required.
// A stream can bind one or more subjects which almost always include
// wildcards. In addition, no two streams can have overlapping subjects,
// otherwise the primary messages would be persisted twice.
var config = new StreamConfig(name: "EVENTS", subjects: ["events.>"]);

// JetStream provides both file and in-memory storage options. For
// durability of the stream data, file storage must be chosen to
// survive crashes and restarts. This is the default for the stream,
// but we can still set it explicitly.
config.Storage = StreamConfigStorage.File;

// Finally, let's add/create the stream with the default (no) limits.
var stream = await js.CreateStreamAsync(config);

// Let's publish a few messages which are received by the stream since
// they match the subject bound to the stream. The `js.Publish` method
// is a convenience for sending a `Request` and waiting for the
// acknowledgement.
for (var i = 0; i < 2; i++)
{
    await js.PublishAsync<int>(subject: "events.page_loaded", data: 1);
    await js.PublishAsync<int>(subject: "events.mouse_clicked", data: 1);
    await js.PublishAsync<int>(subject: "events.mouse_clicked", data: 1);
    await js.PublishAsync<int>(subject: "events.page_loaded", data: 2);
    await js.PublishAsync<int>(subject: "events.mouse_clicked", data: 2);
    await js.PublishAsync<int>(subject: "events.input_focused", data: 1);
    Console.WriteLine("Published 6 messages");
}

// Checking out the stream info, we can see how many messages we
// have.
await PrintStreamStateAsync(stream);

// Stream configuration can be dynamically changed. For example,
// we can set the max messages limit to 10, and it will truncate the
// two initial events in the stream.
var configUpdate = config with { MaxMsgs = 10 };
await js.UpdateStreamAsync(configUpdate);
Console.WriteLine("set max messages to 10");

// Checking out the info, we see there are now 10 messages and the
// first sequence and timestamp are based on the third message.
await PrintStreamStateAsync(stream);

// Limits can be combined, and whichever one is reached, it will
// be applied to truncate the stream. For example, let's set a
// maximum number of bytes for the stream.
configUpdate.MaxBytes = 300;
await js.UpdateStreamAsync(configUpdate);
Console.WriteLine("set max bytes to 300");

// Inspecting the stream info, we now see more messages have been
// truncated to ensure the size is not exceeded.
await PrintStreamStateAsync(stream);

// Finally, for the last primary limit, we can set the max age.
configUpdate.MaxAge = TimeSpan.FromSeconds(1);
await js.UpdateStreamAsync(configUpdate);
Console.WriteLine("set max age to one second");

// Looking at the stream info, we still see all the messages.
await PrintStreamStateAsync(stream);

// until a second passes.
Console.WriteLine("sleeping one second...");
await Task.Delay(TimeSpan.FromSeconds(1));

await PrintStreamStateAsync(stream);
    
// That's it!
Console.WriteLine("Bye!");

async Task PrintStreamStateAsync(INatsJSStream jsStream)
{
    await jsStream.RefreshAsync();
    var state = jsStream.Info.State;
    Console.WriteLine($"Stream has {state.Messages} messages using {state.Bytes} bytes");
}
