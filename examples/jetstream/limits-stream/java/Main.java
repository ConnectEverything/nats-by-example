package example;

import io.nats.client.*;
import io.nats.client.api.PublishAck;
import io.nats.client.api.StorageType;
import io.nats.client.api.StreamConfiguration;
import io.nats.client.api.StreamInfo;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CompletableFuture;

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

            // We will declare the initial stream configuration by specifying
            // the name and subjects. Stream names are commonly uppercased to
            // visually differentiate them from subjects, but this is not required.
            // A stream can bind one or more subjects which almost always include
            // wildcards. In addition, no two streams can have overlapping subjects
            // otherwise the primary messages would be persisted twice. There
            // are options to replicate messages in various ways, but that will
            // be explained in later examples.
            //
            // JetStream provides both file and in-memory storage options. For
            // durability of the stream data, file storage must be chosen to
            // survive crashes and restarts. This is the default for the stream,
            // but we can still set it explicitly.
            String streamName = "EVENTS";
            StreamConfiguration.Builder streamConfigBuilder = StreamConfiguration.builder()
                    .name(streamName)
                    .subjects("events.>")
                    .storageType(StorageType.File);

            // Finally, let's add/create the stream with the default (no) limits.
            StreamInfo stream = jsm.addStream(streamConfigBuilder.build());
            System.out.println("Created the stream.");

            // Let's publish a few messages which are received by the stream since
            // they match the subject bound to the stream. The `js.publish` method
            // is a convenience for sending a `nc.request` and waiting for the
            // acknowledgement.
            js.publish("events.page_loaded", null);
            js.publish("events.mouse_clicked", null);
            js.publish("events.mouse_clicked", null);
            js.publish("events.page_loaded", null);
            js.publish("events.mouse_clicked", null);
            js.publish("events.input_focused", null);
            System.out.println("Published 6 messages.");

            // There is also an async form in which the client batches the
            // messages to the server and then asynchronously receives
            // the acknowledgements.
            List<CompletableFuture<PublishAck>> futures = new ArrayList<>();
            futures.add(js.publishAsync("events.input_changed", null));
            futures.add(js.publishAsync("events.input_blurred", null));
            futures.add(js.publishAsync("events.key_pressed", null));
            futures.add(js.publishAsync("events.input_focused", null));
            futures.add(js.publishAsync("events.input_changed", null));
            futures.add(js.publishAsync("events.input_blurred", null));

            CompletableFuture<Void> publishCompleted = CompletableFuture.allOf(futures.toArray(new CompletableFuture[0]));
            try {
                publishCompleted.join();
                System.out.println("Published 6 messages.");
            } catch (Exception e) {
                System.out.println("Publish failed " + e);
            }

            // Checking out the stream info, we can see how many messages we
            // have.
            printStreamState(jsm, streamName);

            // Stream configuration can be dynamically changed. For example,
            // we can set the max messages limit to 10 and it will truncate the
            // two initial events in the stream.
            streamConfigBuilder.maxMessages(10);
            jsm.updateStream(streamConfigBuilder.build());
            System.out.println("Set max messages to 10.");

            // Checking out the info, we see there are now 10 messages and the
            // first sequence and timestamp are based on the third message.
            printStreamState(jsm, streamName);

            // Limits can be combined and whichever one is reached, it will
            // be applied to truncate the stream. For example, let's set a
            // maximum number of bytes for the stream.
            streamConfigBuilder.maxBytes(300);
            jsm.updateStream(streamConfigBuilder.build());
            System.out.println("Set max bytes to 300.");

            // Inspecting the stream info we now see more messages have been
            // truncated to ensure the size is not exceeded.
            printStreamState(jsm, streamName);

            // Finally, for the last primary limit, we can set the max age.
            streamConfigBuilder.maxAge(Duration.ofSeconds(1));
            jsm.updateStream(streamConfigBuilder.build());
            System.out.println("Set max age to one second.");

            // Looking at the stream info, we still see all the messages..
            printStreamState(jsm, streamName);

            // until a second passes.
            System.out.println("Sleeping one second...");
            Thread.sleep(1000);

            printStreamState(jsm, streamName);
        } catch (InterruptedException | IOException | JetStreamApiException e) {
            e.printStackTrace();
        }
    }

    private static void printStreamState(JetStreamManagement jsm, String streamName) throws IOException, JetStreamApiException {
        StreamInfo streamInfo = jsm.getStreamInfo(streamName);
        System.out.println("Inspecting stream info:");
        System.out.println(streamInfo.getStreamState());
    }
}
