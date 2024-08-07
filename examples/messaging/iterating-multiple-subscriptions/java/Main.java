package example;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Nats;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
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

            int total = 80;
            CountDownLatch latch = new CountDownLatch(total);

            // Create a message dispatcher. A dispatcher is a process that runs on
            // its own thread, receives incoming messages via a FIFO queue,
            // for subjects registered on it. For each message it takes from
            // the queue, it makes a blocking call to the MessageHandler
            // passed to the createDispatcher call.
            Dispatcher dispatcher = nc.createDispatcher((msg) -> {
                System.out.printf("Received %s: %s\n",
                        msg.getSubject(),
                        new String(msg.getData(), StandardCharsets.UTF_8));
                latch.countDown();
            });

            // Subscribe directly on the dispatcher for multiple subjects.
            dispatcher.subscribe("s1");
            dispatcher.subscribe("s2");
            dispatcher.subscribe("s3");
            dispatcher.subscribe("s4");

            for (int i = 0; i < total / 4; i++) {
                nc.publish("s1", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s2", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s3", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s4", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                Thread.sleep(100);
            }

            // Await the dispatcher thread to have received all the messages before the program quits.
            latch.await();

        } catch (InterruptedException | IOException e) {
            e.printStackTrace();
        }
    }
}
