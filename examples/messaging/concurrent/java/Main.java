package example;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Nats;
import io.nats.client.Options;

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

            int total = 50;
            CountDownLatch latch = new CountDownLatch(total);

            // Create message dispatchers with queue groups for handling messages in
            // separate threads.
            for (int i = 0; i < 4; i++) {
                Dispatcher dispatcher = nc.createDispatcher((msg) -> {
                    System.out.printf("Received %s\n",
                            new String(msg.getData(), StandardCharsets.UTF_8));
                    latch.countDown();
                });

                dispatcher.subscribe("greet", "queue");
            }

            for (int i = 0; i < total; i++) {
                nc.publish("greet", String.format("hello %s", i).getBytes(StandardCharsets.UTF_8));
            }

            // Await the dispatcher threads to have received all the messages before the program quits.
            latch.await();

        } catch (InterruptedException | IOException e) {
            e.printStackTrace();
        }
    }
}
