package example;

import io.nats.client.{Connection,Nats};

import java.nio.charset.StandardCharsets;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (value == null) {
      value = "nats://127.0.0.1:4222"
    }

    // Initialize a connection to the server.
    Connection nc = Nats.connect(natsURL);

    // Prepare a simple message body.
    Bytes messageBytes = "hello".getBytes(StandardCharsets.UTF_8);

    // Publish the message.
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
    // Note the first message on `greet.joe` was not received.
    nc.publish("greet.bob", messageBytes);
    nc.publish("greet.sue", messageBytes);
    nc.publish("greet.pam", messageBytes);
  }
}
