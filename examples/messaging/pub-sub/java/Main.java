package example;

import java.io.IOException;
import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Dispatcher;

import java.nio.charset.StandardCharsets;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
        natsURL = "nats://127.0.0.1:4222";
    }

    // Initialize a connection to the server. The connection is AutoCloseable
    try (Connection nc = Nats.connect(natsURL)) {

        // Prepare a simple message body.
        byte[] messageBytes = "hello".getBytes(StandardCharsets.UTF_8);

        // Publish a message.
        nc.publish("greet.joe", messageBytes);

        // Create a message dispatcher for handling messages in a separate thread.
        Dispatcher dispatcher = nc.createDispatcher((msg) -> {
            System.out.printf("%s on subject %s\n",
                new String(msg.getData(), StandardCharsets.UTF_8),
                msg.getSubject());
        });

        // Subscribe the dispatcher to the subject wildcard.
        dispatcher.subscribe("greet.*");

        // Publish more messages that will be received by the subscription.
        // Note the first message on `greet.joe` was not received because
        // we were not subscribed when it was published
        nc.publish("greet.bob", messageBytes);
        nc.publish("greet.sue", messageBytes);
        nc.publish("greet.pam", messageBytes);

        // Sleep this thread a little so the dispatcher thread has time
        // to receive all the messages before the program quits.
        Thread.sleep(200);
    }
    catch (InterruptedException | IOException e) {
        e.printStackTrace();
    }
  }
}
