package example;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.Message;
import io.nats.client.Nats;

import java.io.IOException;
import java.time.Duration;
import java.util.concurrent.*;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    // Initialize a connection to the server. The connection is AutoCloseable
    // on exit.
    try (Connection nc = Nats.connect(natsURL)) {

      // ### Reply
      // Create a message dispatcher for handling messages in a
      // separate thread and then subscribe to the target subject
      // which leverages a wildcard `greet.*`.
      // When a user makes a "request", the client populates
      // the reply-to field and then listens (subscribes) to that
      // as a subject.
      // The replier simply publishes a message to that reply-to.
      Dispatcher dispatcher = nc.createDispatcher((msg) -> {
        String name = msg.getSubject().substring(6);
        String response = "hello " + name;
        nc.publish(msg.getReplyTo(), response.getBytes());
      });
      dispatcher.subscribe("greet.*");

      // ### Request
      // Make a request and wait a most 1 second for a response.
      Message m = nc.request("greet.bob", null, Duration.ofSeconds(1));
      System.out.println("Response received: " + new String(m.getData()));

      // A request can also be made asynchronously.
      try {
        CompletableFuture<Message> future = nc.request("greet.pam", null);
        m = future.get(1, TimeUnit.SECONDS);
        System.out.println("Response received: " + new String(m.getData()));
      } catch (ExecutionException e) {
        System.out.println("Something went wrong with the execution of the request: " + e);
      } catch (TimeoutException e) {
        System.out.println("We didn't get a response in time.");
      } catch (CancellationException e) {
        System.out.println("The request was cancelled due to no responders.");
      }

      // Once we unsubscribe there will be no subscriptions to reply.
      dispatcher.unsubscribe("greet.*");

      // If there are no-responders to a synchronous request
      // we just time out and get a null response.
      m = nc.request("greet.fred", null, Duration.ofMillis(300));
      System.out.println("Response was null? " + (m == null));

      // If there are no-responders to an asynchronous request
      // we get a cancellation exception.
      try {
        CompletableFuture<Message> future = nc.request("greet.sue", null);
        m = future.get(1, TimeUnit.SECONDS);
        System.out.println("Response received: " + new String(m.getData()));
      } catch (ExecutionException e) {
        System.out.println("Something went wrong with the execution of the request: " + e);
      } catch (TimeoutException e) {
        System.out.println("We didn't get a response in time.");
      } catch (CancellationException e) {
        System.out.println("The request was cancelled due to no responders.");
      }

    } catch (InterruptedException | IOException e) {
      e.printStackTrace();
    }
  }
}
