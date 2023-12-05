// Install NuGet packages `NATS.Net` and `Microsoft.Extensions.Logging.Console`.

using System;
using System.Text;
using System.Text.Json.Serialization;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using NATS.Client.Core;

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

await nats.ConnectAsync();

// When subscribing or publishing you can use the generated JSON serializer to deserialize the JSON payload.
// We can also demonstrate how to use the raw JSON payload and how to use a binary data.
// For more information about the serializers see see our [documentation](https://nats-io.github.io/nats.net/serializers/).
var mySerializer = new NatsJsonContextSerializer<MyData>(MyJsonContext.Default);

var subIterator1 = await nats.SubscribeCoreAsync<MyData>("data", serializer: mySerializer);

var subTask1 = Task.Run(async () =>
{
    logger.LogInformation("Waiting for messages...");
    await foreach (var msg in subIterator1.Msgs.ReadAllAsync())
    {
        if (msg.Data is null)
        {
            logger.LogInformation("Received empty payload: End of messages");
            break;
        }
        var data = msg.Data;
        logger.LogInformation("Received deserialized object {Data}", data);
    }
});

var subIterator2 = await nats.SubscribeCoreAsync<NatsMemoryOwner<byte>>("data");

var subTask2 = Task.Run(async () =>
{
    logger.LogInformation("Waiting for messages...");
    await foreach (var msg in subIterator2.Msgs.ReadAllAsync())
    {
        using var memoryOwner = msg.Data;
        
        if (memoryOwner.Length == 0)
        {
            logger.LogInformation("Received empty payload: End of messages");
            break;
        }

        var json = Encoding.UTF8.GetString(memoryOwner.Span);
        
        logger.LogInformation("Received raw JSON {Json}", json);
    }
});

await nats.PublishAsync<MyData>(subject: "data", data: new MyData{ Id = 1, Name = "Bob" }, serializer: mySerializer);
await nats.PublishAsync<byte[]>(subject: "data", data: Encoding.UTF8.GetBytes("""{"id":2,"name":"Joe"}"""));

var alice = """{"id":3,"name":"Alice"}""";
var bw = new NatsBufferWriter<byte>();
var byteCount = Encoding.UTF8.GetByteCount(alice);
var memory = bw.GetMemory(byteCount);
Encoding.UTF8.GetBytes(alice, memory.Span);
bw.Advance(byteCount);
await nats.PublishAsync<NatsBufferWriter<byte>>(subject: "data", data: bw);

await nats.PublishAsync(subject: "data");

await Task.WhenAll(subTask1, subTask2);

// That's it!
logger.LogInformation("Bye!");

// ## Serializer generator
[JsonSerializable(typeof(MyData))]
internal partial class MyJsonContext : JsonSerializerContext;

public record MyData
{
    [JsonPropertyName("id")]
    public int Id { get; set; }

    [JsonPropertyName("name")]
    public string? Name { get; set; }
}
