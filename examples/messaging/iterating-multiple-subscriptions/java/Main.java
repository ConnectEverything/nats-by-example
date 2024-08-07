package example;

import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Dispatcher;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

public class Main {
    public static void main(String[] args) {
        String natsURL = System.getenv("NATS_URL");
        if (natsURL == null) {
            natsURL = "nats://127.0.0.1:4222";
        }

        // Initialize a connection to the server. The connection is AutoCloseable
        // on exit.
        try (Connection nc = Nats.connect(natsURL)) {

            // Create a message dispatcher for handling messages in a
            // separate thread and then subscribe to multiple target subjects.
            Dispatcher dispatcher = nc.createDispatcher((msg) -> {
                System.out.printf("Received %s: %s\n",
                        msg.getSubject(),
                        new String(msg.getData(), StandardCharsets.UTF_8));
            });

            dispatcher.subscribe("s1");
            dispatcher.subscribe("s2");
            dispatcher.subscribe("s3");
            dispatcher.subscribe("s4");

            int total = 80;
            for (int i = 0; i < total / 4; i++)
            {
                nc.publish("s1", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s2", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s3", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                nc.publish("s4", String.valueOf(i).getBytes(StandardCharsets.UTF_8));
                Thread.sleep(100);
            }

            // Sleep this thread a little so the dispatcher thread has time
            // to receive all the messages before the program quits.
            Thread.sleep(200);

        } catch (InterruptedException | IOException e) {
            e.printStackTrace();
        }
    }
}
