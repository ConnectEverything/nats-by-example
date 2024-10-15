// Install `NATS.Net` from NuGet.
using System.Diagnostics;
using NATS.Client.Core;
using NATS.Net;

var stopwatch = Stopwatch.StartNew();

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "127.0.0.1:4222";
Log($"[CON] Connecting to {url}...");

// Connect to NATS server.
// Since connection is disposable at the end of our scope, we should flush
// our buffers and close the connection cleanly.
await using var nc = new NatsClient(url);

// Create a message event handler and then subscribe to the target
// subject which leverages a wildcard `greet.*`.
// When a user makes a "request", the client populates
// the reply-to field and then listens (subscribes) to that
// as a subject.
// The responder simply publishes a message to that reply-to.
var cts = new CancellationTokenSource();
 var responder = Task.Run(async () =>
 {
     await foreach (var msg in nc.SubscribeAsync<int>("greet.*").WithCancellation(cts.Token))
     {
         var name = msg.Subject.Split('.')[1];
         Log($"[REP] Received {msg.Subject}");
         await Task.Delay(500);
         await msg.ReplyAsync($"Hello {name}!");
     }
 });

// Make a request and wait a most 1 second for a response.
var replyOpts = new NatsSubOpts { Timeout = TimeSpan.FromSeconds(2) };

Log("[REQ] From joe");
var reply = await nc.RequestAsync<int, string>("greet.joe", 0, replyOpts: replyOpts);
Log($"[REQ] {reply.Data}");

Log("[REQ] From sue");
reply = await nc.RequestAsync<int, string>("greet.sue", 0, replyOpts: replyOpts);
Log($"[REQ] {reply.Data}");

Log("[REQ] From bob");
reply = await nc.RequestAsync<int, string>("greet.bob", 0, replyOpts: replyOpts);
Log($"[REQ] {reply.Data}");

// Once we unsubscribe, there will be no subscriptions to reply.
await cts.CancelAsync();

await responder;

// Now there is no responder our request will time out.

try
{
    reply = await nc.RequestAsync<int, string>("greet.joe", 0, replyOpts: replyOpts);
    Log($"[REQ] {reply.Data} - We should not see this message.");
}
catch (NatsNoRespondersException)
{
    Log("[REQ] no responders!");
}

// That's it! We saw how we can create a responder and request data from it. We also set
// request timeouts to make sure we can move on when there is no response to our requests.
Log("Bye!");

return;

void Log(string log) => Console.WriteLine($"{stopwatch.Elapsed} {log}");