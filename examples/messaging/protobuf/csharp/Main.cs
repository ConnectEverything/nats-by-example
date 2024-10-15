// Install NuGet packages `NATS.Net` and `Google.Protobuf`
using System.Buffers;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using NATS.Client.Core;

// `NATS_URL` environment variable can be used to pass the locations of the NATS servers.
var url = Environment.GetEnvironmentVariable("NATS_URL") ?? "nats://127.0.0.1:4222";

// Connect to NATS server.
// Since connection is disposable at the end of our scope, we should flush
// our buffers and close the connection cleanly.
// Notice the use of custom serializer registry.
var opts = new NatsOpts
{
    Url = url,
    SerializerRegistry = new MyProtoBufSerializerRegistry(),
    Name = "NATS-by-Example",
};
await using var nats = new NatsConnection(opts);

// Subscribe to a subject and start waiting for messages in the background.
// Notice that we are using a custom serializer for the subscription.
var sub = Task.Run(async () =>
{
    Console.WriteLine("Waiting for messages...");
    await foreach (var msg in nats.SubscribeAsync<GreetRequest>(subject: "greet", serializer: MyProtoBufSerializer<GreetRequest>.Default))
    {
        if (msg.Data is null)
        {
            Console.WriteLine("Received empty payload: End of messages");
            break;
        }
        var request = msg.Data;
        var reply = new GreetReply { Text = $"hello {request.Name}"};
        await msg.ReplyAsync(reply, serializer: MyProtoBufSerializer<GreetReply>.Default);
    }
});

// This request uses the default serializer for the connection assigned to connection options above.
// Alternatively, we could've passed the individual serializer to the request method.
var reply = await nats.RequestAsync<GreetRequest, GreetReply>(subject: "greet", new GreetRequest { Name = "bob" });
Console.WriteLine($"Response = {reply.Data?.Text}...");

// Send an empty message to indicate we are done.
await nats.PublishAsync("greet");

// We can unsubscribe now all orders are published. Unsubscribing or disposing the subscription
// should complete the message loop and exit the background task cleanly.
await sub;

// That's it!
Console.WriteLine("Bye!");

// ## Serializer Registry
public class MyProtoBufSerializerRegistry : INatsSerializerRegistry
{
    public INatsSerialize<T> GetSerializer<T>() => MyProtoBufSerializer<T>.Default;

    public INatsDeserialize<T> GetDeserializer<T>() => MyProtoBufSerializer<T>.Default;
}

// ## Serializer
public class MyProtoBufSerializer<T> : INatsSerializer<T>
{
    public static readonly INatsSerializer<T> Default = new MyProtoBufSerializer<T>();

    public void Serialize(IBufferWriter<byte> bufferWriter, T value)
    {
        if (value is IMessage message)
        {
            message.WriteTo(bufferWriter);
        }
        else
        {
            throw new NatsException($"Can't serialize {typeof(T)}");
        }
    }

    public T? Deserialize(in ReadOnlySequence<byte> buffer)
    {
        if (buffer.Length == 0)
        {
            /* serializers define what to do with empty payloads. */
            return default;
        }
        
        if (typeof(T) == typeof(GreetRequest))
        {
            return (T)(object)GreetRequest.Parser.ParseFrom(buffer);
        }

        if (typeof(T) == typeof(GreetReply))
        {
            return (T)(object)GreetReply.Parser.ParseFrom(buffer);
        }

        throw new NatsException($"Can't deserialize {typeof(T)}");
    }

    public INatsSerializer<T> CombineWith(INatsSerializer<T> next)
    {
        throw new NotImplementedException();
    }
}

// ## Protobuf Messages
// The following messages would normally be generated using `protoc`.
// For the sake of example,
// we defined simplified versions here. Usually you would use `.proto` files to define your
// messages and generate the code using `Grpc.Tools` NuGet package to generate them in a separate
// project. See [gRPC documentation](https://grpc.io/docs/languages/csharp/basics/) and
// [ASP.NET Core Tooling Support](https://learn.microsoft.com/en-us/aspnet/core/grpc/basics?view=aspnetcore-8.0#c-tooling-support-for-proto-files)
// for more details.
public class GreetRequest : IMessage<GreetRequest>, IBufferMessage
{
    public static readonly MessageParser<GreetRequest> Parser = new(() => new GreetRequest());
    
    public string? Name { get; set; }

    public void MergeFrom(GreetRequest message) => Name = message.Name;

    public void MergeFrom(CodedInputStream input)
    {
        uint tag;
        while ((tag = input.ReadTag()) != 0) {
            if (tag == 10)
                Name = input.ReadString();
        }
    }

    public void WriteTo(CodedOutputStream output)
    {
        output.WriteRawTag(10);
        output.WriteString(Name);
    }

    public int CalculateSize() => CodedOutputStream.ComputeStringSize(Name) + 1;

    public MessageDescriptor Descriptor => null!;

    public bool Equals(GreetRequest? other) => string.Equals(other?.Name, Name);

    public GreetRequest Clone() => new() { Name = Name };

    public void InternalMergeFrom(ref ParseContext input)
    {
        uint tag;
        while ((tag = input.ReadTag()) != 0) {
            if (tag == 10)
                Name = input.ReadString();
        }
    }

    public void InternalWriteTo(ref WriteContext output)
    {
        output.WriteRawTag(10);
        output.WriteString(Name);
    }
}

public class GreetReply : IMessage<GreetReply>, IBufferMessage
{
    public static readonly MessageParser<GreetReply> Parser = new(() => new GreetReply());
    
    public string? Text { get; set; }

    public void MergeFrom(GreetReply message) => Text = message.Text;

    public void MergeFrom(CodedInputStream input)
    {
        uint tag;
        while ((tag = input.ReadTag()) != 0) {
            if (tag == 10)
                Text = input.ReadString();
        }
    }

    public void WriteTo(CodedOutputStream output)
    {
        output.WriteRawTag(10);
        output.WriteString(Text);
    }

    public int CalculateSize() => CodedOutputStream.ComputeStringSize(Text) + 1;

    public MessageDescriptor Descriptor => null!;

    public bool Equals(GreetReply? other) => string.Equals(other?.Text, Text);

    public GreetReply Clone() => new() { Text = Text };
    
    public void InternalMergeFrom(ref ParseContext input)
    {
        uint tag;
        while ((tag = input.ReadTag()) != 0) {
            if (tag == 10)
                Text = input.ReadString();
        }
    }

    public void InternalWriteTo(ref WriteContext output)
    {
        output.WriteRawTag(10);
        output.WriteString(Text);
    }
}
