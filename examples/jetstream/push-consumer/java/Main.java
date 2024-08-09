package example;

import io.nats.client.*;
import io.nats.client.api.*;
import io.nats.client.impl.NatsJetStreamMetaData;

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

            // Declare a simple [limits-based stream][1] and populate the stream
            // with a few messages.
            // [1]: /examples/jetstream/limits-stream/java/
            String streamName = "EVENTS";
            StreamConfiguration config = StreamConfiguration.builder()
                    .name(streamName)
                    .subjects("events.>")
                    .build();

            StreamInfo stream = jsm.addStream(config);

            js.publish("events.1", null);
            js.publish("events.2", null);
            js.publish("events.3", null);

            // ### Ephemeral
            // The JetStream context provides a simple way to create an ephemeral
            // push consumer, simply provide a subject that overlaps with the
            // bound subjects on the stream and this helper method will do the
            // stream look-up automatically and create the consumer.
            System.out.println("# Ephemeral");
            Subscription sub = js.subscribe("events.>");

            // An ephemeral consumer has a name generated on the server-side.
            // Since there is only one consumer so far, let's just get the first
            // one.
            String ephemeralName = jsm.getConsumerNames(streamName).get(0);
            System.out.printf("Ephemeral name is '%s'\n", ephemeralName);

            // Since this is a push consumer, messages will be sent by the server
            // and pre-buffered by this subscription. We can observe this by using
            // the `getPendingMessageCount()` method. Messages are buffered asynchronously, so
            // this pending count may or may not be three.
            long queuedMsgs = sub.getPendingMessageCount();
            System.out.printf("%d messages queued\n", queuedMsgs);

            // The maximum number of messages that will be queued is defined by the
            // `MaxAckPending` option set on a consumer. The default is 1,000.
            // Let's observe this by publishing a few more events and then check
            // the pending status again.
            js.publish("events.4", null);
            js.publish("events.5", null);
            js.publish("events.6", null);

            // Let's check if we buffered some more.
            queuedMsgs = sub.getPendingMessageCount();
            System.out.printf("%d messages queued\n", queuedMsgs);

            // To *receive* a message, call `nextMsg` with a timeout. The timeout
            // applies when pending count is zero and the consumer has fully caught
            // up to the available messages in the stream. If no messages become
            // available, this call will only block until the timeout.
            Message msg = sub.nextMessage(Duration.ofSeconds(1));
            System.out.printf("Received '%s'\n", msg.getSubject());

            // By default, the underlying consumer requires explicit acknowledgements,
            // otherwise messages will get redelivered.
            msg.ack();

            // Let's receive and ack another.
            msg = sub.nextMessage(Duration.ofSeconds(1));
            System.out.printf("Received '%s'\n", msg.getSubject());
            msg.ack();

            // Checking out our pending information, we see there are no more
            // than four remaining.
            queuedMsgs = sub.getPendingMessageCount();
            System.out.printf("%d messages queued\n", queuedMsgs);

            // Unsubscribing this subscription will result in the ephemeral consumer
            // being deleted. Note, even if this is omitted and the process ends
            // or is interrupted, the server will eventually clean-up the ephemeral
            // when it determines the subscription is no longer active.
            sub.unsubscribe();

            // ### Durable (AddConsumer)
            // An explicit and safer way to create consumers is using `jsm.addOrUpdateConsumer`.
            // For push consumers, we must provide a `DeliverSubject` which is the
            // subject messages will be published to (pushed) for a subscription to
            // receive them.
            //
            // Important: such a short AckWait should not be used, it's simply for demonstration purposes in this example.
            System.out.println("\n# Durable (AddConsumer)");

            String consumerName = "handler-1";
            ConsumerConfiguration cc = ConsumerConfiguration.builder()
                    .durable(consumerName)
                    .deliverSubject(consumerName)
                    .ackPolicy(AckPolicy.Explicit)
                    .ackWait(Duration.ofSeconds(1))
                    .build();
            jsm.addOrUpdateConsumer(streamName, cc);

            // Now that the consumer is created, we need to bind a client subscription
            // to it which will receive and process the messages. This can be done
            // using the `PushSubscribeOptions.bind` option which requires the consumer
            // to have been pre-created. The subject can be omitted since that was
            // already defined on the consumer. Subscriptions to consumers cannot
            // independently define their own subject to filter on.
            PushSubscribeOptions so = PushSubscribeOptions.bind(streamName, consumerName);
            sub = js.subscribe(null, so);

            // The next step is to receive a message which can be done using
            // the `nextMessage` method. The passed duration is the amount of time
            // to wait before until a message is received. This is received because
            // `SubscribeSync` is the _synchronous_ form of a push consumer
            // subscription. There is also the `Subscribe` variant which takes
            // a `nats.MsgHandler` function to receive and process messages
            // asynchronously, but that will be described in a different example.
            msg = sub.nextMessage(Duration.ofSeconds(1));
            System.out.printf("Received '%s'\n", msg.getSubject());

            // Let's ack the message and check out the pending count which will have
            // a few buffered as shown above.
            msg.ack();
            queuedMsgs = sub.getPendingMessageCount();
            System.out.printf("%d messages queued\n", queuedMsgs);

            // If we unsubscribe, what happens to these pending messages?
            // From the client's perspective they are effectively dropped. This behavior
            // would be true if the client crashed for some reason.
            // From the server's perspective it is going to wait until `AckWait`
            // before attempting to re-deliver them. However, it will only re-deliver
            // if there is an active subscription.
            sub.unsubscribe();

            // If we check out the consumer info, we can pull out a few interesting
            // bits of information. The first one is that the consumer tracks the
            // sequence of the last message in the _stream_ that a delivery was
            // attempted for. The second is that it maintains its own sequence to
            // track delivery attempts. These should not be treated as correlated
            // since the consumer sequence for a given message will increment on
            // each delivery attempt.
            // The "num ack pending" indicates how many messages have been delivered
            // and awaiting an acknowledgement. Since we ack'ed one already, there
            // are five remaining.
            // The final one to note here are the number of redeliveries. Since
            // these messages have been only delivered once (so far) for this consumer
            // this value is zero.
            ConsumerInfo info = jsm.getConsumerInfo(streamName, consumerName);
            System.out.printf("Max stream sequence delivered: %d\n", info.getDelivered().getStreamSequence());
            System.out.printf("Max consumer sequence delivered: %d\n", info.getDelivered().getConsumerSequence());
            System.out.printf("Num ack pending: %d\n", info.getNumAckPending());
            System.out.printf("Num redelivered: %d\n", info.getRedelivered());

            // If we create a new subscription and attempt to get a message
            // before the AckWait, we will get a timeout since the messages
            // are still pending.
            sub = js.subscribe(null, so);
            msg = sub.nextMessage(Duration.ofMillis(100));
            System.out.printf("Received timeout? %b\n", msg == null);

            // Let's try again and wait a bit longer beyond the AckWait. We
            // can also see that the delivery attempt on the message is now 2.
            msg = sub.nextMessage(Duration.ofSeconds(1));
            NatsJetStreamMetaData md = msg.metaData();
            System.out.printf("Received '%s' (delivery #%d)\n", msg.getSubject(), md.deliveredCount());
            msg.ack();

            // We can see how the numbers changed by viewing the consumer info
            // again.
            info = jsm.getConsumerInfo(streamName, consumerName);
            System.out.printf("Max stream sequence delivered: %d\n", info.getDelivered().getStreamSequence());
            System.out.printf("Max consumer sequence delivered: %d\n", info.getDelivered().getConsumerSequence());
            System.out.printf("Num ack pending: %d\n", info.getNumAckPending());
            System.out.printf("Num redelivered: %d\n", info.getRedelivered());
        } catch (InterruptedException | IOException | JetStreamApiException e) {
            e.printStackTrace();
        }
    }
}
