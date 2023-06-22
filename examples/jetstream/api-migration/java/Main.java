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
      // ## JetStream / JetStreamManagement contexts
      //
      // * JetStream/JetStreamManagement contexts are created from the Connection.
      // * JetStreamManagement context is used for things like stream and consumer management.
      // * JetStream context is used for publishing and subscribing
      // * If a consumer does not exist when subscribe is called, one will be created.
      JetStream js = conn.jetStream();
      JetStreamManagement jsm = conn.jetStreamManagement();

      // Create a stream and populate the stream with a few messages.
      String streamName = "migration";
      if (jsm.getStreamNames().contains(streamName)) {
        jsm.deleteStream(streamName); // just want a clean stream for the example.
      }
      jsm.addStream(StreamConfiguration.builder()
          .name(streamName)
          .storageType(StorageType.Memory)
          .subjects("events.>")
          .build());

      js.publish("events.1", null);
      js.publish("events.2", null);
      js.publish("events.3", null);

      // ### Continuous message retrieval with `subscribe()`
      //
      // Using the JetStream context, the common way to continuously receive messages is to use push consumers.
      // The easiest way to create a consumer and start consuming messages
      // using the JetStream context is to use the `subscribe()` method. `subscribe()`,
      // while familiar to core NATS users, leads to complications because it will
      // create underlying consumers if they don't already exist.
      System.out.println("\nA. Legacy Push Subscription with Ephemeral Consumer");

      System.out.println("  Async");
      Dispatcher dispatcher = conn.createDispatcher();

      // By default, `subscribe()` performs stream lookup by subject.
      // You can save a lookup to the server by providing the stream name in the subscribe options
      PushSubscribeOptions pushSubscribeOptions = PushSubscribeOptions.stream(streamName);

      JetStreamSubscription sub = js.subscribe("events.>", dispatcher,
          msg -> {
            System.out.println("      Received " + msg.getSubject());
            msg.ack();
          }, false, pushSubscribeOptions);
      Thread.sleep(100);

      // Unsubscribing this subscription will result in the underlying
      // ephemeral consumer being deleted faster on the server
      dispatcher.unsubscribe(sub);

      System.out.println("  Sync");
      sub = js.subscribe("events.>", pushSubscribeOptions);
      while (true) {
        Message msg = sub.nextMessage(100);
        if (msg == null) {
          break;
        }
        System.out.println("      Read " + msg.getSubject());
        msg.ack();
      }
      sub.unsubscribe();

      // ### Binding to an existing consumer
      //
      // In order to create a consumer outside the `subscribe` method,
      // the JetStreamManagement context `addOrUpdateConsumer` method can be used.
      // If a durable is not provided, the consumer will be ephemeral and will
      // be deleted if it becomes inactive for longer than the inactive threshold.
      // If durable and name are not provided, the client will generate a name
      // that can be found via `ConsumerInfo.getName()`
      System.out.println("\nB. Legacy Bind Subscription to Named Consumer.");
      ConsumerConfiguration consumerConfiguration = ConsumerConfiguration.builder()
          .deliverSubject("deliverB") // required for push consumers!
          .ackPolicy(AckPolicy.Explicit)
          .inactiveThreshold(Duration.ofMinutes(10))
          .build();
      ConsumerInfo consumerInfo = jsm.addOrUpdateConsumer(streamName, consumerConfiguration);
      sub = js.subscribe(null, dispatcher,
          msg -> {
            System.out.println("   Received " + msg.getSubject());
            msg.ack();
          }, false, PushSubscribeOptions.bind(streamName, consumerInfo.getName()));

      Thread.sleep(100);
      dispatcher.unsubscribe(sub);

      // ### Pull consumers
      //
      // The JetStream context api also supports pull consumers.
      // Using pull consumers requires more effort on the developer's side
      // than push consumers to maintain an endless stream of messages.
      // Messages can be retrieved using the `iterate` method.
      // Iterate will start retrieving messages from the server as soon as
      // it is called but returns right away (does not block) so you can
      // start handling messages as soon as the first one comes from the server.
      System.out.println("\nC. Legacy Pull Subscription then Iterate");
      PullSubscribeOptions pullSubscribeOptions = PullSubscribeOptions.builder().build();
      sub = js.subscribe("events.>", pullSubscribeOptions);
      long start = System.currentTimeMillis();
      Iterator<Message> iterator = sub.iterate(10, 2000);
      long elapsed = System.currentTimeMillis() - start;
      System.out.println("   The call to `iterate(10, 2000)` returned in " + elapsed + "ms.");
      while (iterator.hasNext()) {
        Message msg = iterator.next();
        elapsed = System.currentTimeMillis() - start;
        System.out.println("   Processing " + msg.getSubject() + " " + elapsed + "ms after start.");
        msg.ack();
      }
      elapsed = System.currentTimeMillis() - start;
      System.out.println("   The iterate completed in " + elapsed + "ms.\n" +
          "       Time reflects waiting for the entire batch, which isn't available.");

      // ## Simplified Stream and Consumer API
      //
      // The simplified api has a StreamContext for accessing existing
      // streams, creating consumers and creating a ConsumerContext.
      // The StreamContext can be created from the Connection using the same
      // JetStreamOptions used to create the JetStream/Management contexts
      // From the StreamContext, you could get a ConsumerContext. You could
      // also get a ConsumerContext from the Connection if you don't need
      // to do anything with the stream.
      System.out.println("\nD. Simplification StreamContext");
      StreamContext streamContext = conn.streamContext(streamName);
      StreamInfo streamInfo = streamContext.getStreamInfo(StreamInfoOptions.allSubjects());
      System.out.println("   Stream Name: " + streamInfo.getConfiguration().getName());
      System.out.println("   Stream Subjects: " + streamInfo.getStreamState().getSubjects());

      // ### Creating a consumer from the stream context
      //
      // This is the most basic consumer configuration
      // * The server will assign the consumer a name.
      // * The filter subject will be equivalent to ">" or all subjects in the stream.
      // * The inactive threshold defaults to 5 seconds.
      System.out.println("\nE. Simplification, Create a Consumer");
      consumerConfiguration = ConsumerConfiguration.builder().build();
      ConsumerContext consumerContext = streamContext.createOrUpdateConsumer(consumerConfiguration);
      consumerInfo = consumerContext.getCachedConsumerInfo();
      System.out.println("   A consumer was created on stream \"" + consumerInfo.getStreamName() + "\"");
      System.out.println("   The consumer name is \"" + consumerInfo.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo.getNumPending() + " messages available.");

      // ### Continuous message retrieval with `consume()`
      //
      // In order to continuously receive messages, the `consume` method
      // can be used with or without a MessageHandler.
      // These methods work similarly to the push `subscribe` methods used to receive messages.
      //
      // `consume` (and other ConsumerContext methods) never create a consumer
      // instead always using a consumer created previously.


      // #### MessageConsumer
      // A `MessageConsumer` is returned when you call the `consume` method on the `ConsumerContext`
      // that accepts a `MessageHandler`.
      // Auto ack is no longer an option when a handler is provided,
      // it is the developer's responsibility to ack or not based on the consumer's ack policy.
      // Ack policy is "explicit" if not otherwise set.
      //
      // Remember, when you have a handler and message are sent asynchronously,
      // make sure you have set up your error handler.
      System.out.println("\nF. MessageConsumer (endless consumer with handler)");
      consumerConfiguration = ConsumerConfiguration.builder().build();
      consumerContext = streamContext.createOrUpdateConsumer(consumerConfiguration);
      consumerInfo = consumerContext.getCachedConsumerInfo();
      System.out.println("   A consumer was created on stream \"" + consumerInfo.getStreamName() + "\"");
      System.out.println("   The consumer name is \"" + consumerInfo.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo.getNumPending() + " messages available.");

      MessageConsumer messageConsumer = consumerContext.consume(
          msg -> {
            System.out.println("   Received " + msg.getSubject());
            msg.ack();
          });
      Thread.sleep(100);

      // `consume` returns a `MessageConsumer` which can be used
      // to stop consuming messages. In contrast to `unsubscribe()` in the
      // legacy API, this will not delete the consumer.
      // The consumer will be automatically deleted by the server when the
      // `inactiveThreshold` is reached.
      messageConsumer.stop(100);
      System.out.println("   stop was called.");

      // #### IterableConsumer
      System.out.println("\nG. IterableConsumer (endless consumer manually calling next)");

      // An `IterableConsumer` is returned when you call the `consume` method on the `ConsumerContext`
      // without supplying a message handler.
      consumerConfiguration = ConsumerConfiguration.builder().build();
      consumerContext = streamContext.createOrUpdateConsumer(consumerConfiguration);
      consumerInfo = consumerContext.getCachedConsumerInfo();
      System.out.println("   A consumer was created on stream \"" + consumerInfo.getStreamName() + "\"");
      System.out.println("   The consumer name is \"" + consumerInfo.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo.getNumPending() + " messages available.");
      IterableConsumer iterableConsumer = consumerContext.consume();
      try {
        for (int x = 0; x < 3; x++) {
          Message msg = iterableConsumer.nextMessage(100);
          System.out.println("   Received " + msg.getSubject());
          msg.ack();
        }
        iterableConsumer.stop(100);
        System.out.println("   stop was called.");
      }

      // Notice the nextMessage method can throw a JetStreamStatusCheckedException
      // Under the covers the `IterableConsumer` is handling more than just messages.
      // It handles information from the server as to the status of the underlying operations.
      // For instance, it is possible, but unlikely, that the consumer could be deleted by another
      // application in your ecosystem and if that happens in the middle of the consumer,
      // the exception would be thrown.
      catch (JetStreamStatusCheckedException se) {
        System.out.println("   JetStreamStatusCheckedException: " + se.getMessage());
      }

      // ### Retrieving messages on demand with `fetch` and `next`

      // #### FetchConsumer
      // A `FetchConsumer` is returned when you call the `fetch` methods on `ConsumerContext`
      // You will use that object to call `nextMessage`
      // Notice there is no stop in the `FetchConsumer` interface, the fetch stops by itself.
      // The new version of fetch is very similar to the old iterate, as it does not block
      // before returning the entire batch.
      System.out.println("\nH. FetchConsumer (bounded consumer)");
      consumerConfiguration = ConsumerConfiguration.builder().build();
      consumerContext = streamContext.createOrUpdateConsumer(consumerConfiguration);
      consumerInfo = consumerContext.getCachedConsumerInfo();
      System.out.println("   A consumer was created on stream \"" + consumerInfo.getStreamName() + "\"");
      System.out.println("   The consumer name is \"" + consumerInfo.getName() + "\".");
      System.out.println("   The consumer has " + consumerInfo.getNumPending() + " messages available.");

      start = System.currentTimeMillis();
      FetchConsumer fetchConsumer = consumerContext.fetchMessages(2);
      elapsed = System.currentTimeMillis() - start;
      System.out.println("   'fetch' returned in " + elapsed + "ms.");

      // `fetch` will return null once there are no more messages to consume.
      try {
        Message msg = fetchConsumer.nextMessage();
        while (msg != null) {
          elapsed = System.currentTimeMillis() - start;
          System.out.println("   Processing " + msg.getSubject() + " " + elapsed + "ms after start.");
          msg.ack();
          msg = fetchConsumer.nextMessage();
        }
      }
      catch (JetStreamStatusCheckedException se) {
        System.out.println("   JetStreamStatusCheckedException: " + se.getMessage());
      }
      elapsed = System.currentTimeMillis() - start;
      System.out.println("   Fetch complete in " + elapsed + "ms.");

      // #### next
      // The `next` method can be used to retrieve a single
      // message, as if you had called the old fetch or iterate with a batch size of 1.
      // The minimum wait time when calling next is 1 second (1000ms)
      System.out.println("\nI. next (1 message)");
      try {
        Message msg = consumerContext.next(1000);
        System.out.println("   Received " + msg.getSubject());
        msg.ack();
      }
      catch (JetStreamStatusCheckedException se) {
        System.out.println("   JetStreamStatusCheckedException: " + se.getMessage());
      }
    }
    catch (JetStreamApiException | IOException | InterruptedException e) {
      // JetStreamApiException:
      //      the stream or consumer did not exist
      // IOException:
      //      problem making the connection
      // InterruptedException:
      //      thread interruption in the body of the example
      System.out.println(e);
    }
  }
}