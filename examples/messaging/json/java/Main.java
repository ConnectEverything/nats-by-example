package example;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Dispatcher;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

public class Main {

    public static void main(String[] args) {
        String natsURL = System.getenv("NATS_URL");
        if (natsURL == null) {
            natsURL = "nats://127.0.0.1:4222";
        }

        // Initialize a connection to the server. The connection is AutoCloseable
        // on exit.
        try (Connection nc = Nats.connect(natsURL)) {

            // Construct a payload and serialize it (using Jackson).
            ObjectMapper objectMapper = new ObjectMapper();
            Payload payload = new Payload("bar", 27);
            byte[] messageBytes = objectMapper.writeValueAsBytes(payload);

            // Create a message dispatcher for handling messages in a
            // separate thread and then subscribe to the target subject.
            Dispatcher dispatcher = nc.createDispatcher((msg) -> {

                // Attempt to deserialize the payload.
                // If deserialization fails, alternate handling can be performed.
                try {
                    Payload deserializedPayload = objectMapper.readValue(msg.getData(), Payload.class);

                    System.out.printf("received valid JSON payload: %s\n", deserializedPayload);
                } catch (IOException e) {
                    System.out.printf("received invalid JSON payload: %s\n",
                            new String(msg.getData(), StandardCharsets.UTF_8));
                }
            });

            dispatcher.subscribe("foo");

            // Publish the serialized payload.
            nc.publish("foo", messageBytes);
            nc.publish("foo", "not json".getBytes(StandardCharsets.UTF_8));

            // Sleep this thread a little so the dispatcher thread has time
            // to receive all the messages before the program quits.
            Thread.sleep(200);

        } catch (InterruptedException | IOException e) {
            e.printStackTrace();
        }
    }
}

class Payload {
    private final String foo;
    private final int bar;

    @JsonCreator
    Payload(@JsonProperty("foo") String foo,
            @JsonProperty("bar") int bar) {
        this.foo = foo;
        this.bar = bar;
    }

    public String getFoo() {
        return foo;
    }

    public int getBar() {
        return bar;
    }

    @Override
    public String toString() {
        return "Payload{" +
                "foo='" + foo + '\'' +
                ", bar=" + bar +
                '}';
    }
}
