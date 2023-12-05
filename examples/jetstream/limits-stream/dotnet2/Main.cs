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

// Create `JetStream Context` which provides methods to create
// streams and consumers as well as convenience methods for publishing
// to streams and consuming messages from the streams.
var js = new NatsJSContext(nats);

// We will declare the initial stream configuration by specifying
// the name and subjects. Stream names are commonly uppercase to
// visually differentiate them from subjects, but this is not required.
// A stream can bind one or more subjects which almost always include
// wildcards. In addition, no two streams can have overlapping subjects
// otherwise the primary messages would be persisted twice.
var config = new StreamConfig(name: "EVENTS", subjects: new [] { "events.>" });

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
    await js.PublishAsync<object>(subject: "events.page_loaded", data: null);
    await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);
    await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);
    await js.PublishAsync<object>(subject: "events.page_loaded", data: null);
    await js.PublishAsync<object>(subject: "events.mouse_clicked", data: null);
    await js.PublishAsync<object>(subject: "events.input_focused", data: null);
    logger.LogInformation("Published 6 messages");
}

// Checking out the stream info, we can see how many messages we
// have.
await PrintStreamStateAsync(stream);

var configUpdate = new StreamUpdateRequest { Name = config.Name, Subjects = config.Subjects, Storage = config.Storage };

// Stream configuration can be dynamically changed. For example,
// we can set the max messages limit to 10 and it will truncate the
// two initial events in the stream.
configUpdate.MaxMsgs = 10;
await js.UpdateStreamAsync(configUpdate);
logger.LogInformation("set max messages to 10");

// Checking out the info, we see there are now 10 messages and the
// first sequence and timestamp are based on the third message.
await PrintStreamStateAsync(stream);

// Limits can be combined and whichever one is reached, it will
// be applied to truncate the stream. For example, let's set a
// maximum number of bytes for the stream.
configUpdate.MaxBytes = 300;
await js.UpdateStreamAsync(configUpdate);
logger.LogInformation("set max bytes to 300");

// Inspecting the stream info we now see more messages have been
// truncated to ensure the size is not exceeded.
await PrintStreamStateAsync(stream);

// Finally, for the last primary limit, we can set the max age.
configUpdate.MaxAge = (long)TimeSpan.FromSeconds(1).TotalNanoseconds;
await js.UpdateStreamAsync(configUpdate);
logger.LogInformation("set max age to one second");

// Looking at the stream info, we still see all the messages..
await PrintStreamStateAsync(stream);

// until a second passes.
logger.LogInformation("sleeping one second...");
await Task.Delay(TimeSpan.FromSeconds(1));

await PrintStreamStateAsync(stream);
    
// That's it!
logger.LogInformation("Bye!");

async Task PrintStreamStateAsync(INatsJSStream jsStream)
{
    await jsStream.RefreshAsync();
    var state = jsStream.Info.State;
    logger.LogInformation("Stream has {Messages} messages using {Bytes} bytes", state.Messages, state.Bytes);
}
