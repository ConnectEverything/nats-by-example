package example;

import io.nats.client.*;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.ConsumerInfo;
import io.nats.client.api.StorageType;
import io.nats.client.api.StreamConfiguration;

import java.io.IOException;
import java.time.Duration;
import java.util.concurrent.TimeoutException;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    try (Connection nc = Nats.connect(natsURL)) {
      JetStreamManagement jsm = nc.jetStreamManagement();
      JetStream js = jsm.jetStream();

      // Create a stream
      // (remove the stream first so we have a clean starting point)
      try { jsm.deleteStream("verifyAckStream"); } catch (JetStreamApiException e) {}

      jsm.addStream(StreamConfiguration.builder()
          .name("verifyAckStream")
          .subjects("verifyAckSubject")
          .storageType(StorageType.Memory)
          .build());

      // Publish a couple messages so we can look at the state
      js.publish("verifyAckSubject", "A".getBytes());
      js.publish("verifyAckSubject", "B".getBytes());

      // Consume a message with 2 different consumers
      // The first consumer will (regular) ack without confirmation
      // The second consumer will ackSync which confirms that ack was handled.
      StreamContext sc = nc.getStreamContext("verifyAckStream");
      ConsumerContext cc1 = sc.createOrUpdateConsumer(ConsumerConfiguration.builder().filterSubject("verifyAckSubject").build());
      ConsumerContext cc2 = sc.createOrUpdateConsumer(ConsumerConfiguration.builder().filterSubject("verifyAckSubject").build());

      // Consumer 1 will use ack()
      ConsumerInfo ci = cc1.getConsumerInfo();
      System.out.println("Consumer 1");
      System.out.println("  Start\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      // Get one message with ConsumerContext next()
      Message m = cc1.next();
      ci = cc1.getConsumerInfo();
      System.out.println("  After received but before ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      // ack the message.
      m.ack();
      ci = cc1.getConsumerInfo();
      System.out.println("  After ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      // Consumer 2 will use ackAck()
      ci = cc2.getConsumerInfo();
      System.out.println("Consumer 2");
      System.out.println("  Start\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      // Get one message with ConsumerContext next()
      m = cc2.next();
      ci = cc2.getConsumerInfo();
      System.out.println("  After received but before ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      // ackSync the message.
      // The thing about ackSync is it is a request reply.
      // Make a request to the server, the server does work, the server replies.
      // It's rare, but the request could get to the server,
      // the server handles it and sends the response, but it
      // does not make it back to the client in time. This is where
      // knowledge of the environment and network is important
      // when setting connection and request time-outs.
      try {
        m.ackSync(Duration.ofMillis(500));
      }
      catch (TimeoutException timeout) {
        System.out.println("If it gets here the server did not reply in time.");
      }
      ci = cc2.getConsumerInfo();
      System.out.println("  After ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());
    }
    catch (InterruptedException | IOException | JetStreamApiException | JetStreamStatusCheckedException e) {
      e.printStackTrace();
    }
  }
}
