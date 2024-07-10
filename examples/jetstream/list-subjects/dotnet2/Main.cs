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

// ### GetStreamInfo with StreamInfoOptions
// Get the subjects via the getStreamInfo call.
// Since this is "state" there are no subjects in the state unless
// there are messages in the subject.

// Use the &gt; to filter for all subjects
var jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine($"Before publishing any messages, there are 0 subjects: {jsStream.Info.State.Subjects?.Count}");

// Publish a message
await js.PublishAsync("plain", "plain-data");

jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine("After publishing a message to a subject, it appears in state:");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var subj in jsStream.Info.State.Subjects.Keys)
    {
        Console.WriteLine($"  Subject '{subj}', Count {jsStream.Info.State.Subjects[subj]}");
    }
}

// Publish some more messages, this time against wildcard subjects
await js.PublishAsync("greater.A", "gtA");
await js.PublishAsync("greater.A.B", "gtAB");
await js.PublishAsync("greater.A.B.C", "gtABC");
await js.PublishAsync("greater.B.B.B", "gtBBB");
await js.PublishAsync("star.1", "star1");
await js.PublishAsync("star.2", "star2");

jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = ">" });
Console.WriteLine("Wildcard subjects show the actual subject, not the template:");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var subj in jsStream.Info.State.Subjects.Keys)
    {
        Console.WriteLine($"  Subject '{subj}', Count {jsStream.Info.State.Subjects[subj]}");
    }
}

// ### Subject Filtering
// Instead of allSubjects, you can filter for a specific subject
jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = "greater.>" });
Console.WriteLine("Filtering the subject returns only matching entries ['greater.>']");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var subj in jsStream.Info.State.Subjects.Keys)
    {
        Console.WriteLine($"  Subject '{subj}', Count {jsStream.Info.State.Subjects[subj]}");
    }
}

jsStream = await js.GetStreamAsync(stream, new StreamInfoRequest() { SubjectsFilter = "greater.A.>" });
Console.WriteLine("Filtering the subject returns only matching entries ['greater.A.>']");
if (jsStream.Info.State.Subjects != null)
{
    foreach (var subj in jsStream.Info.State.Subjects.Keys)
    {
        Console.WriteLine($"  Subject '{subj}', Count {jsStream.Info.State.Subjects[subj]}");
    }
}
