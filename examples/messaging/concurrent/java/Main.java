package example;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Nats;
import io.nats.client.Options;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

public class Main {
    public static void main(String[] args) {
        String natsURL = System.getenv("NATS_URL");
        if (natsURL == null) {
            natsURL = "nats://127.0.0.1:4222";
        }

        // Setup options using a Dispatcher with executor.
        // A custom executor can be specified if desired, using `.executor(..)`.
        Options options = Options.builder()
                .server(natsURL)
                .useDispatcherWithExecutor()
                .build();

        // Initialize a connection to the server. The connection is AutoCloseable
        // on exit.
        try (Connection nc = Nats.connect(options)) {

            // Create a message dispatcher for handling messages in
            // separate threads.
            Dispatcher dispatcher = nc.createDispatcher((msg) -> {
                System.out.printf("Received %s\n",
                        new String(msg.getData(), StandardCharsets.UTF_8));
            });

            dispatcher.subscribe("greet");

            for (int i = 0; i < 50; i++) {
                nc.publish("greet", String.format("hello %s", i).getBytes(StandardCharsets.UTF_8));
            }

            // Sleep this thread a little so the dispatcher thread has time
            // to receive all the messages before the program quits.
            Thread.sleep(200);

        } catch (InterruptedException | IOException e) {
            e.printStackTrace();
        }
    }
}
