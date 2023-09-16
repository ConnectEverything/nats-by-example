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

    try (Connection conn = Nats.connect(natsURL)) {
      System.out.println("\nA. Prepare Example Stream and Consumers");

      // ## The JetStream and JetStreamManagement
      //
      // The `JetStreamManagement` context provides the ability
      // to create and manage streams.
      // The `JetStream` context provides the ability
      // to publish messages.
      JetStream js = conn.jetStream();
      JetStreamManagement jsm = conn.jetStreamManagement();

      // Create a stream and populate the stream with a few messages.
      String streamName = "fetch";
      jsm.addStream(StreamConfiguration.builder()
          .name(streamName)
          .storageType(StorageType.Memory)
          .subjects("events.>")
          .build());

      // publish some messages to the stream
      js.publish("events.1", "e1m1".getBytes());
      js.publish("events.2", "e2m1".getBytes());
      js.publish("events.1", "e1m2".getBytes());
      js.publish("events.2", "e2m2".getBytes());

      // Although you can make consumers on the fly, typically
      // consumers will be created ahead of time.
      ConsumerConfiguration cc = ConsumerConfiguration.builder().name("onlyEvents1").filterSubject("events.1").build();
      jsm.addOrUpdateConsumer(streamName, cc);
      cc = ConsumerConfiguration.builder().name("allEvents").filterSubject("events.*").build();
      jsm.addOrUpdateConsumer(streamName, cc);

      // ## Simplified JetStream API
      //
      // The simplified API has a `StreamContext` for accessing existing
      // streams, creating consumers, and getting a `ConsumerContext`.
      // The `StreamContext` can be created from the `Connection` similar to
      // the legacy API.
      System.out.println("\nB. Use Simplification StreamContext");
      StreamContext streamContext = conn.getStreamContext(streamName);
      StreamInfo streamInfo = streamContext.getStreamInfo(StreamInfoOptions.allSubjects());

      System.out.println("   Stream Name: " + streamInfo.getConfiguration().getName());
      System.out.println("   Stream Subjects: " + streamInfo.getStreamState().getSubjects());
      System.out.println("   Stream Message Count: " + streamInfo.getStreamState().getMsgCount());

      // ### Creating a consumer from the stream context
      //
      // To create an ephemeral consumer, the `createOrUpdateConsumer` method
      // can be used with a bare `ConsumerConfiguration` object.

      // ### Getting a consumer from the stream context
      //
      // If your consumer already exists as a durable, you can create a
      // `ConsumerContext` for that consumer from the stream context or directly
      // from the connection by providing the stream and consumer name.
      System.out.println("\nC. Simplification Consumer Context");
      ConsumerContext consumerContext1 = streamContext.getConsumerContext("onlyEvents1");
      ConsumerInfo consumerInfo1 = consumerContext1.getCachedConsumerInfo();

      System.out.println("   The ConsumerContext for \"" + consumerInfo1.getName() + "\" was loaded from the StreamContext for \"" + consumerInfo1.getStreamName() + "\"");
      System.out.println("   The consumer has " + consumerInfo1.getNumPending() + " messages available.");

      ConsumerContext consumerContext2 = streamContext.getConsumerContext("allEvents");
      ConsumerInfo consumerInfo2 = consumerContext2.getCachedConsumerInfo();

      System.out.println("\n   The ConsumerContext for \"" + consumerInfo2.getName() + "\" was loaded from the StreamContext for \"" + consumerInfo2.getStreamName() + "\"");
      System.out.println("   The consumer has " + consumerInfo2.getNumPending() + " messages available.");

      // ### Retrieving messages on demand with `fetch`

      // #### FetchConsumer
      // A `FetchConsumer` is returned when you call the `fetch` methods on `ConsumerContext`.
      // You will use that object to call `nextMessage`.
      // Notice there is no stop on the `FetchConsumer` interface, the fetch stops by itself.
      // The new version of fetch is very similar to the old iterate, as it does not block
      // before returning the entire batch.
      System.out.println("\nD. FetchConsumer");
      System.out.println("   The consumer name is \"" + consumerInfo1.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo1.getNumPending() + " messages available.");

      long start = System.currentTimeMillis();
      long elapsed;
      try (FetchConsumer fetchConsumer = consumerContext1.fetchMessages(2)) {
        elapsed = System.currentTimeMillis() - start;
        System.out.println("   The 'fetch' method call returned in " + elapsed + "ms.");

        // `fetch` will return null once there are no more messages to consume.
        try {
          Message msg = fetchConsumer.nextMessage();
          while (msg != null) {
            String data = new String(msg.getData());
            System.out.println("   Processing " + msg.getSubject() + " '" + data);
            msg.ack();
            msg = fetchConsumer.nextMessage();
          }
        }
        catch (JetStreamStatusCheckedException se) {
          System.out.println("   JetStreamStatusCheckedException: " + se.getMessage());
        }
      }
      catch (Exception e) {
        throw new RuntimeException(e);
      }
      elapsed = System.currentTimeMillis() - start;
      System.out.println("   Fetch complete in " + elapsed + "ms.");

      System.out.println("\n   The consumer name is \"" + consumerInfo2.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo2.getNumPending() + " messages available.");

      start = System.currentTimeMillis();
      try (FetchConsumer fetchConsumer = consumerContext2.fetchMessages(2)) {
        elapsed = System.currentTimeMillis() - start;
        System.out.println("   The 'fetch' method call returned in " + elapsed + "ms.");

        // `fetch` will return null once there are no more messages to consume.
        try {
          Message msg = fetchConsumer.nextMessage();
          while (msg != null) {
            elapsed = System.currentTimeMillis() - start;
            String data = new String(msg.getData());
            System.out.println("   Processing " + msg.getSubject() + " '" + data);
            msg.ack();
            msg = fetchConsumer.nextMessage();
          }
        }
        catch (JetStreamStatusCheckedException se) {
          System.out.println("   JetStreamStatusCheckedException: " + se.getMessage());
        }
      }
      catch (Exception e) {
        throw new RuntimeException(e);
      }
      elapsed = System.currentTimeMillis() - start;
      System.out.println("   Fetch complete in " + elapsed + "ms.");
    }
    catch (JetStreamApiException | IOException | InterruptedException e) {
      // * JetStreamApiException: the stream or consumer did not exist
      // * IOException: problem making the connection
      // * InterruptedException: thread interruption in the body of the example
      System.err.println(e);
    }
  }
}
