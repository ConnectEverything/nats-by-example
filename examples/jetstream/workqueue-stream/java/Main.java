package example;

import io.nats.client.*;
import io.nats.client.api.StreamConfiguration;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.StreamInfo;
import io.nats.client.api.StreamState;
import io.nats.client.api.RetentionPolicy;
import io.nats.client.api.AckPolicy;

import java.io.IOException;
import java.time.Duration;
import java.nio.charset.StandardCharsets;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
        natsURL = "nats://127.0.0.1:4222";
    }

    // Initialize a connection to the server. The connection is AutoCloseable on exit.
    try (Connection nc = Nats.connect(natsURL)) {

      // ### Creating the stream
      // Initialize the `JetStreamManagement` context for managing JetStream
      // assets.
      JetStreamManagement jsm = nc.jetStreamManagement();

      // Define the stream configuration, specifying the retention policy
      // as `WorkQueue` and create it.
      StreamConfiguration sc = StreamConfiguration.builder()
        .name("EVENTS")
        .subjects("events.>")
        .retentionPolicy(RetentionPolicy.WorkQueue)
        .build();

      jsm.addStream(sc);
      System.out.printf("Created stream %s\n", sc.getName());

      // ### Queue
      // Create a `JetStream` context for publishing and subscribing.
      JetStream js = nc.jetStream();

      js.publish("events.us.page_loaded", null);
      js.publish("events.eu.mouse_clicked", null);
      js.publish("events.us.input_focused", null);
      System.out.println("Published 3 messages");

      StreamInfo info = jsm.getStreamInfo("EVENTS");
      long msgCount = info.getStreamState().getMsgCount();
      System.out.printf("msgs in stream: %s\n", msgCount);

      // ### Adding a consumer
      // Now let's add a consumer. It can be a *push* or *pull* consumer,
      // however for this example we will use *pull*.
      // The first pull consumer will be bound to all subjects in
      // the stream. 
      ConsumerConfiguration c1 = ConsumerConfiguration.builder()
        .durable("processor-1")
        .ackPolicy(AckPolicy.Explicit)
        .build();

      jsm.addOrUpdateConsumer("EVENTS", c1);

      // Now we bind the subscription. Note the empty string for the subject
      // implies interest in all subjects in the stream.
      PullSubscribeOptions o1 = PullSubscribeOptions.bind("EVENTS", "processor-1");
      JetStreamSubscription s1 = js.subscribe("", o1);

      // Now we can fetch all three messages in the stream.
      s1.pull(3);
      for (int i = 0; i < 3; i++) {
        Message msg = s1.nextMessage(Duration.ofSeconds(1));
        msg.ack();
        System.out.printf("Received event: %s\n", msg.getSubject());
      }
      s1.unsubscribe();

      // We can confirm by checking the stream state again.
      info = jsm.getStreamInfo("EVENTS");
      msgCount = info.getStreamState().getMsgCount();
      System.out.printf("msgs in stream: %s\n", msgCount);

      // ### Exclusive non-filtered consumer
      // As noted in the description above, work-queue streams can only
      // have at most one consumer with interest on a subject at any given
      // time. Since this first pull consumer is not filtered, it we try
      // to create another consumer, it will fail.
      ConsumerConfiguration c2 = ConsumerConfiguration.builder()
        .durable("processor-2")
        .ackPolicy(AckPolicy.Explicit)
        .build();

      try {
        jsm.addOrUpdateConsumer("EVENTS", c2);
      } catch (JetStreamApiException e) {
        System.out.println("Failed to create an overlapping consumer");
      }

      // However if we delete the first consumer, this will clean up
      // the first consumer since it was an ephemeral, which removes interest.
      jsm.deleteConsumer("EVENTS", "processor-1");
      System.out.println("Deleting the first consumer");

      jsm.addOrUpdateConsumer("EVENTS", c2);
      System.out.println("Succeeded to create a new consumer");
      jsm.deleteConsumer("EVENTS", "processor-2");

      // ### Multiple filtered consumers
      // To create multiple consumers, we need to filter the subjects.
      // For example, we can create a consumer that only receives
      // subjects that start with `events.us` and another that only
      // receives subjects that start with `events.eu`.
      ConsumerConfiguration c3 = ConsumerConfiguration.builder()
        .durable("processor-3")
        .ackPolicy(AckPolicy.Explicit)
        .filterSubject("events.us.>")
        .build();

      jsm.addOrUpdateConsumer("EVENTS", c3);

      PullSubscribeOptions o3 = PullSubscribeOptions.bind("EVENTS", "processor-3");

      JetStreamSubscription s3 = js.subscribe("", o3);

      ConsumerConfiguration c4 = ConsumerConfiguration.builder()
        .durable("processor-4")
        .ackPolicy(AckPolicy.Explicit)
        .filterSubject("events.eu.>")
        .build();

      jsm.addOrUpdateConsumer("EVENTS", c4);

      PullSubscribeOptions o4 = PullSubscribeOptions.bind("EVENTS", "processor-4");

      JetStreamSubscription s4 = js.subscribe("", o4);
      System.out.println("Created two filtered consumers");

      js.publish("events.us.page_loaded", null);
      js.publish("events.eu.mouse_clicked", null);
      js.publish("events.us.input_focused", null);
      System.out.println("Published 3 more messages");

      s3.pull(2);
      for (int i = 0; i < 2; i++) {
        Message msg = s3.nextMessage(Duration.ofSeconds(1));
        msg.ack();
        System.out.printf("Received event via 'processor-us': %s\n", msg.getSubject());
      }

      s4.pull(1);
      Message msg = s4.nextMessage(Duration.ofSeconds(1));
      msg.ack();
      System.out.printf("Received event via 'processor-eu': %s\n", msg.getSubject());

      s3.unsubscribe();
      jsm.deleteConsumer("EVENTS", "processor-3");

      s4.unsubscribe();
      jsm.deleteConsumer("EVENTS", "processor-4");

      // Sleep this thread a little so the dispatcher thread has time
      // to receive all the messages before the program quits.
      Thread.sleep(200);

    } catch (Exception e) {
      e.printStackTrace();
    }
  }
}
