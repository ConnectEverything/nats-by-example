package example;

import io.nats.client.*;
import io.nats.client.api.*;

import java.io.IOException;
import java.time.Duration;
import java.util.Iterator;
import java.util.List;
import java.util.concurrent.CompletableFuture;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    try (Connection nc = Nats.connect(natsURL)) {
      JetStreamManagement jsm = nc.jetStreamManagement();
      JetStream js = jsm.jetStream();

      // remove the stream so we have a clean starting point
      try {
        jsm.deleteStream("verifyAckStream");
      }
      catch (JetStreamApiException e) {
        // means does not exist
      }

      // Create a stream with a few subjects
      jsm.addStream(StreamConfiguration.builder()
          .name("verifyAckStream")
          .subjects("verifyAckSubject")
          .storageType(StorageType.Memory)
          .build());

      // Publish a message
      js.publish("verifyAckSubject", "A".getBytes());
      js.publish("verifyAckSubject", "B".getBytes());

      // Consume the message
      StreamContext sc = nc.getStreamContext("verifyAckStream");
      ConsumerContext cc1 = sc.createOrUpdateConsumer(ConsumerConfiguration.builder().filterSubject("verifyAckSubject").build());
      ConsumerContext cc2 = sc.createOrUpdateConsumer(ConsumerConfiguration.builder().filterSubject("verifyAckSubject").build());

      ConsumerInfo ci = cc1.getConsumerInfo();
      System.out.println("Consumer 1");
      System.out.println("  Start\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      Message m = cc1.next();
      ci = cc1.getConsumerInfo();
      System.out.println("  After received but before ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      m.ack();
      Thread.sleep(100); // to give time for the ack to be completed on the server
      ci = cc1.getConsumerInfo();
      System.out.println("  After ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());


      ci = cc2.getConsumerInfo();
      System.out.println("Consumer 2");
      System.out.println("  Start\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      m = cc2.next();
      ci = cc2.getConsumerInfo();
      System.out.println("  After received but before ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());

      m.ackSync(Duration.ofMillis(500));
      ci = cc2.getConsumerInfo();
      System.out.println("  After ack\n    # pending messages: " + ci.getNumPending() + "\n    # messages with ack pending: " + ci.getNumAckPending());
    } catch (InterruptedException | IOException | JetStreamApiException | JetStreamStatusCheckedException | TimeoutException e) {
      e.printStackTrace();
    }
  }
}
