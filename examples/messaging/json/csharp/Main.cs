// Install NuGet package `NATS.Net`
using System.Text;
using System.Text.Json.Serialization;
using NATS.Client.Core;
using NATS.Net;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server. Since connection is disposable at the end of our scope we should flush
// our buffers and close connection cleanly.
await using var nc = new NatsClient(url);

await nc.ConnectAsync();

// When subscribing or publishing you can use the generated JSON serializer to deserialize the JSON payload.
// We can also demonstrate how to use the raw JSON payload and how to use a binary data.
// For more information about the serializers see see our [documentation](https://nats-io.github.io/nats.net/serializers/).
var mySerializer = new NatsJsonContextSerializer<MyData>(MyJsonContext.Default);

var subTask1 = Task.Run(async () =>
{
    Console.WriteLine("Waiting for messages...");
    await foreach (var msg in nc.SubscribeAsync<MyData>("data", serializer: mySerializer))
    {
        if (msg.Data is null)
        {
            Console.WriteLine("Received empty payload: End of messages");
            break;
        }
        var data = msg.Data;
        Console.WriteLine($"Received deserialized object {data}");
    }
});

var subTask2 = Task.Run(async () =>
{
    Console.WriteLine("Waiting for messages...");
    await foreach (var msg in nc.SubscribeAsync<NatsMemoryOwner<byte>>("data"))
    {
        using var memoryOwner = msg.Data;
        
        if (memoryOwner.Length == 0)
        {
            Console.WriteLine("Received empty payload: End of messages");
            break;
        }

        var json = Encoding.UTF8.GetString(memoryOwner.Span);
        
        Console.WriteLine($"Received raw JSON {json}");
    }
});

// Give some time for the subscriptions to start.
await Task.Delay(1000);

await nc.PublishAsync<MyData>(subject: "data", data: new MyData{ Id = 1, Name = "Bob" }, serializer: mySerializer);
await nc.PublishAsync<byte[]>(subject: "data", data: Encoding.UTF8.GetBytes("""{"id":2,"name":"Joe"}"""));

var alice = """{"id":3,"name":"Alice"}""";
var bw = new NatsBufferWriter<byte>();
var byteCount = Encoding.UTF8.GetByteCount(alice);
var memory = bw.GetMemory(byteCount);
Encoding.UTF8.GetBytes(alice, memory.Span);
bw.Advance(byteCount);
await nc.PublishAsync<NatsBufferWriter<byte>>(subject: "data", data: bw);

// End of messages published as an empty payload.
await nc.PublishAsync(subject: "data");

await Task.WhenAll(subTask1, subTask2);

// That's it!
Console.WriteLine("Bye!");

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
