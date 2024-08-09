package example;

import io.nats.client.*;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.StreamConfiguration;
import io.nats.client.api.StreamInfo;

import java.io.IOException;
import java.time.Duration;
import java.util.concurrent.CountDownLatch;

public class Main {
    public static void main(String[] args) {
        String natsURL = System.getenv("NATS_URL");
        if (natsURL == null) {
            natsURL = "nats://127.0.0.1:4222";
        }

        // Initialize a connection to the server. The connection is AutoCloseable
        // on exit.
        try (Connection nc = Nats.connect(natsURL)) {

            // Access `JetStream` and `JetStreamManagement` which provide methods to create
            // streams and consumers as well as convenience methods for publishing
            // to streams and consuming messages from the streams.
            JetStream js = nc.jetStream();
            JetStreamManagement jsm = nc.jetStreamManagement();

            // Declare a simple [limits-based stream](/examples/jetstream/limits-stream/java/).
            String streamName = "EVENTS";
            StreamConfiguration config = StreamConfiguration.builder()
                    .name(streamName)
                    .subjects("events.>")
                    .build();

            StreamInfo stream = jsm.addStream(config);

            // Publish a few messages for the example.
            js.publish("events.1", null);
            js.publish("events.2", null);
            js.publish("events.3", null);

            // Create the consumer bound to the previously created stream. If durable
            // name is not supplied, consumer will be removed after InactiveThreshold
            // (defaults to 5 seconds) is reached when not actively consuming messages.
            // `name` is optional, if not provided it will be auto-generated.
            // For this example, let's use the consumer with no options, which will
            // be ephemeral with auto-generated name.
            StreamContext streamContext = js.getStreamContext(streamName);
            ConsumerContext consumerContext = streamContext.createOrUpdateConsumer(ConsumerConfiguration.builder().build());

            // Messages can be _consumed_  continuously in callback using `consume`
            // method. `consume` can be supplied with various options, but for this
            // example we will use the default ones. WaitGroup is used as part of this
            // example to make sure to stop processing  after we process 3 messages (so
            // that it does not interfere with other examples).
            CountDownLatch latch = new CountDownLatch(3);
            MessageHandler handler = msg -> {
                System.out.printf("Received msg on %s\n", msg.getSubject());
                msg.ack();
                latch.countDown();
            };

            try (MessageConsumer messageConsumer = consumerContext.consume(handler)) {
                latch.await();

                // Consume can be stopped by calling `stop` on the returned MessageConsumer.
                // This will stop the callback from being called and stop retrieving the
                // messages.
                messageConsumer.stop();
            } catch (Exception e) {
                e.printStackTrace();
            }

            // Publish more messages.
            js.publish("events.1", null);
            js.publish("events.2", null);
            js.publish("events.3", null);

            // We can _fetch_ messages in batches. The fetch can have a _maximum_
            // number of messages and/or bytes that should be returned.
            // For this first fetch we ask for two, and we will get
            // those since they are in the stream.
            try (FetchConsumer fetchConsumer = consumerContext.fetchMessages(2)) {
                int count = 0;

                Message msg;
                while ((msg = fetchConsumer.nextMessage()) != null) {
                    // Let's ack the messages so they are not redelivered.
                    msg.ack();
                    count++;
                }

                System.out.printf("Got %d messages\n", count);
                fetchConsumer.stop();
            } catch (Exception e) {
                e.printStackTrace();
            }

            // `fetch` returns the messages by calling `nextMessage` and will return `null`
            // when the requested number of messages have been received or the operation times out.
            // If we do not want to wait for the rest of the messages and want to quickly return
            // as many messages as there are available (up to provided batch size),
            // we can use `noWait` instead.
            // Here, because we have already received two messages, we will only get
            // one more.
            FetchConsumeOptions fetchConsumeOptions = FetchConsumeOptions.builder().noWait().build();
            try (FetchConsumer fetchConsumer = consumerContext.fetch(fetchConsumeOptions)) {
                int count = 0;

                Message msg;
                while ((msg = fetchConsumer.nextMessage()) != null) {
                    msg.ack();
                    count++;
                }

                System.out.printf("Got %d messages\n", count);
                fetchConsumer.stop();
            } catch (Exception e) {
                e.printStackTrace();
            }

            // Finally, if we are at the end of the stream and we call fetch,
            // the call will be blocked until the "expires in" time which is 30
            // seconds by default, but this can be set explicitly as an option.
            long start = System.currentTimeMillis();
            fetchConsumeOptions = FetchConsumeOptions.builder()
                    .expiresIn(Duration.ofSeconds(1).toMillis())
                    .build();

            try (FetchConsumer fetchConsumer = consumerContext.fetch(fetchConsumeOptions)) {
                int count = 0;

                Message msg;
                while ((msg = fetchConsumer.nextMessage()) != null) {
                    msg.ack();
                    count++;
                }

                Duration elapsed = Duration.ofMillis(System.currentTimeMillis() - start);
                System.out.printf("Got %d messages in %s\n", count, elapsed);
                fetchConsumer.stop();
            } catch (Exception e) {
                e.printStackTrace();
            }

            // Durable consumers can be created by specifying the durable name.
            // Durable consumers are not removed automatically regardless of the
            // InactiveThreshold. They can be removed by calling `deleteConsumer`.
            ConsumerConfiguration durableConfig = ConsumerConfiguration.builder()
                    .durable("processor")
                    .build();
            ConsumerContext durableContext = streamContext.createOrUpdateConsumer(durableConfig);

            // Consume and fetch work the same way for durable consumers.
            // But since we only want one message we can simply request the next message as well.
            Message msg = durableContext.next();
            System.out.printf("Received '%s' from durable consumer\n", msg.getSubject());

            // While ephemeral consumers will be removed after InactiveThreshold, durable
            // consumers have to be removed explicitly if no longer needed.
            streamContext.deleteConsumer("processor");

            // Let's try to get the consumer to make sure it's gone.
            try {
                streamContext.getConsumerContext("processor");
            } catch (JetStreamApiException e) {
                System.out.printf("Consumer deleted: %s\n", e.getMessage());
            }
        } catch (InterruptedException | IOException | JetStreamApiException | JetStreamStatusCheckedException e) {
            e.printStackTrace();
        }
    }
}
