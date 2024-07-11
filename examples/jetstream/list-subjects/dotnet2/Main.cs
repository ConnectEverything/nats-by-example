// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.
using NATS.Client.Core;
using NATS.Client.JetStream;
using NATS.Client.JetStream.Models;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
var opts = new NatsOpts
{
    Url = url,
    Name = "NATS-by-Example",
};
await using var nats = new NatsConnection(opts);
var js = new NatsJSContext(nats);

// ### Stream Setup
var stream = "list-subjects";

// Remove the stream first!, so we have a clean starting point.
try
{
    await js.DeleteStreamAsync(stream);
}
catch (NatsJSApiException e) when (e is { Error.Code: 404 })
{
}

// Create the stream with a variety of subjects
var streamConfig = new StreamConfig(stream, ["plain", "greater.>", "star.*"])
{
    Storage = StreamConfigStorage.Memory,
};
await js.CreateStreamAsync(streamConfig);

// ### GetStreamAsync with StreamInfoRequest
// You can get the subjects of a stream via the GetStreamAsync call.
// Since this is "state" a subject is not in the state unless
// there are messages in the subject.
// To get the subjects map, you must provide a SubjectsFilter
// Use the &gt; to filter for all subjects
var jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine($"Before publishing any messages, there are 0 subjects: {jsStream.Info.State.Subjects?.Count}");

// Publish a message
await js.PublishAsync("plain", "plain-data");

// Stream Info contains State, which contains a map, Subjects, of subjects to their count
// but the server only collects and returns that information if
// StreamInfoRequest is present with a non-empty subject filter.
// To get all subjects, set the filter to &gt;
jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine("After publishing a message to a subject, it appears in state:");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var (subject, count) in jsStream.Info.State.Subjects)
    {
        Console.WriteLine($"  Subject '{subject}' has {count} message(s)");
    }
}

// Publish some more messages, this time against wildcard subjects
await js.PublishAsync("greater.A", "gtA-1");
await js.PublishAsync("greater.A", "gtA-2");
await js.PublishAsync("greater.A.B", "gtAB-1");
await js.PublishAsync("greater.A.B", "gtAB-2");
await js.PublishAsync("greater.A.B.C", "gtABC");
await js.PublishAsync("greater.B.B.B", "gtBBB");
await js.PublishAsync("star.1", "star1-1");
await js.PublishAsync("star.1", "star1-2");
await js.PublishAsync("star.2", "star2");

jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine("Wildcard subjects show the actual subject, not the template:");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var (subject, count) in jsStream.Info.State.Subjects)
    {
        Console.WriteLine($"  Subject '{subject}' has {count} message(s)");
    }
}

// ### Specific Subject Filtering
// You can filter for a more specific subject
jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = "greater.>" });
Console.WriteLine("Filtering the subject returns only matching entries ['greater.>']");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var (subject, count) in jsStream.Info.State.Subjects)
    {
        Console.WriteLine($"  Subject '{subject}' has {count} message(s)");
    }
}

jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = "greater.A.>" });
Console.WriteLine("Filtering the subject returns only matching entries ['greater.A.>']");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var (subject, count) in jsStream.Info.State.Subjects)
    {
        Console.WriteLine($"  Subject '{subject}' has {count} message(s)");
    }
}
