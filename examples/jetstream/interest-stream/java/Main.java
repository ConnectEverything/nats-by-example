package example;

import io.nats.client.*;
import io.nats.client.api.*;
import io.nats.client.impl.NatsJetStreamMetaData;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;

public class Main {
    public static void main(String[] args) {
        String natsURL = System.getenv("NATS_URL");
        if (natsURL == null) {
            natsURL = "nats://127.0.0.1:4222";
        }

        // Initialize a connection to the server. The connection is AutoCloseable
        // on exit.
        try (Connection nc = Nats.connect(natsURL)) {

            // Create `JetStream` and `JetStreamManagement` to use the NATS JetStream API.
            // It allows creating and managing streams and consumers as well as
            // publishing to streams and consuming messages from streams.
            JetStream js = nc.jetStream();
            JetStreamManagement jsm = nc.jetStreamManagement();

            // ### Creating the stream
            // Define the stream configuration, specifying `Interest` for retention, and
            // create the stream.
            String streamName = "EVENTS";
            StreamConfiguration config = StreamConfiguration.builder()
                    .name(streamName)
                    .subjects("events.>")
                    .retentionPolicy(RetentionPolicy.Interest)
                    .build();

            StreamInfo stream = jsm.addStream(config);
            System.out.println("Created the stream.");

            // To demonstrate the base case behavior of the stream without any consumers, we
            // will publish a few messages to the stream.
            js.publish("events.page_loaded", null);
            js.publish("events.mouse_clicked", null);
            PublishAck ack = js.publish("events.input_focused", null);
            System.out.println("Published 3 messages.");

            // We confirm that all three messages were published and the last message sequence
            // is 3.
            System.out.printf("Last message seq: %d\n", ack.getSeqno());

            // Checking out the stream info, notice how zero messages are present in
            // the stream, but the `last_seq` is 3 which matches the last ack'ed
            // publish sequence above. Also notice that the `first_seq` is one greater
            // which behaves as a sentinel value indicating the stream is empty. This
            // sequence has not been assigned to a message yet, but can be interpreted
            // as _no messages available_ in this context.
            System.out.println("# Stream info without any consumers");
            printStreamState(jsm, streamName);

            // ### Adding a consumer
            // Now let's add a pull consumer and publish a few
            // more messages. Also note that we are _only_ creating the consumer and
            // have not yet started consuming the messages. This is only to point out
            // that it is not _required_ to be actively consuming messages to show
            // _interest_, but it is the presence of a consumer which the stream cares
            // about to determine retention of messages. [pull]:
            // /examples/jetstream/pull-consumer/java
            StreamContext streamContext = js.getStreamContext(streamName);
            ConsumerContext consumerContext = streamContext.createOrUpdateConsumer(ConsumerConfiguration.builder()
                    .durable("processor-1")
                    .ackPolicy(AckPolicy.Explicit)
                    .build());

            js.publish("events.mouse_clicked", null);
            js.publish("events.input_focused", null);

            // If we inspect the stream info again, we will notice a few differences.
            // It shows two messages (which we expect) and the first and last sequences
            // corresponding to the two messages we just published. We also see that
            // the `consumer_count` is now one.
            System.out.println("\n# Stream info with one consumer");
            printStreamState(jsm, streamName);

            // Now that the consumer is there and showing _interest_ in the messages, we know they
            // will remain until we process the messages. Let's fetch the two messages and ack them.
            try (FetchConsumer fc = consumerContext.fetchMessages(2)) {
                Message m = fc.nextMessage();
                while (m != null) {
                    m.ackSync(Duration.ofSeconds(5));
                    m = fc.nextMessage();
                }
            }

            // What do we expect in the stream? No messages and the `first_seq` has been set to
            // the _next_ sequence number like in the base case.
            // ☝️ As a quick aside on that second ack, We are using `ackSync` here for this
            // example to ensure the stream state has been synced up for this subsequent
            // retrieval.
            System.out.println("\n# Stream info with one consumer and acked messages");
            printStreamState(jsm, streamName);

            // ### Two or more consumers
            // Since each consumer represents a separate _view_ over a stream, we would expect
            // that if messages were processed by one consumer, but not the other, the messages
            // would be retained. This is indeed the case.
            ConsumerContext consumerContext2 = streamContext.createOrUpdateConsumer(ConsumerConfiguration.builder()
                    .durable("processor-2")
                    .ackPolicy(AckPolicy.Explicit)
                    .build());

            js.publish("events.input_focused", null);
            js.publish("events.mouse_clicked", null);

            // Here we fetch 2 messages for `processor-2`. There are two observations to
            // make here. First the fetched messages are the latest two messages that
            // were published just above and not any prior messages since these were
            // already deleted from the stream. This should be apparent now, but this
            // reinforces that a _late_ consumer cannot retroactively show interest. The
            // second point is that the stream info shows that the latest two messages
            // are still present in the stream. This is also expected since the first
            // consumer had not yet processed them.
            List<NatsJetStreamMetaData> messagesMetadata = new ArrayList<>();
            try (FetchConsumer fc = consumerContext2.fetchMessages(2)) {
                Message m = fc.nextMessage();
                while (m != null) {
                    m.ackSync(Duration.ofSeconds(5));
                    messagesMetadata.add(m.metaData());
                    m = fc.nextMessage();
                }
            }

            System.out.printf("Msg seqs %d and %d\n", messagesMetadata.get(0).streamSequence(), messagesMetadata.get(1).streamSequence());

            System.out.println("\n# Stream info with two consumers, but only one set of acked messages");
            printStreamState(jsm, streamName);

            // Fetching and ack'ing from the first consumer subscription will result in the messages
            // being deleted.
            try (FetchConsumer fc = consumerContext.fetchMessages(2)) {
                Message m = fc.nextMessage();
                while (m != null) {
                    m.ackSync(Duration.ofSeconds(5));
                    m = fc.nextMessage();
                }
            }

            System.out.println("\n# Stream info with two consumers having both acked");
            printStreamState(jsm, streamName);

            // A final callout is that _interest_ respects the `FilterSubject` on a consumer.
            // For example, if a consumer defines a filter only for `events.mouse_clicked` events
            // then it won't be considered _interested_ in events such as `events.input_focused`.
            streamContext.createOrUpdateConsumer(ConsumerConfiguration.builder()
                    .durable("processor-3")
                    .ackPolicy(AckPolicy.Explicit)
                    .filterSubject("events.mouse_clicked")
                    .build());

            js.publish("events.input_focused", null);

            // Fetch and term (also works) and ack from the first consumers that _do_ have interest.
            Message m = consumerContext.next();
            m.term();
            m = consumerContext2.next();
            m.ackSync(Duration.ofSeconds(5));

            System.out.println("\n# Stream info with three consumers with interest from two");
            printStreamState(jsm, streamName);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    private static void printStreamState(JetStreamManagement jsm, String streamName) throws IOException, JetStreamApiException {
        StreamInfo streamInfo = jsm.getStreamInfo(streamName);
        System.out.println(streamInfo.getStreamState());
    }
}
