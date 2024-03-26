package example;

import io.nats.client.*;
import io.nats.service.Endpoint;
import io.nats.service.Service;
import io.nats.service.ServiceBuilder;
import io.nats.service.ServiceEndpoint;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.atomic.AtomicBoolean;

public class Main {

  public static void main(String[] args) throws Exception {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    Options options = new Options.Builder()
        .server(natsURL)
        .errorListener(new ErrorListener() {})
        .userInfo("auth", "auth")
        .build();

    try (Connection nc = Nats.connect(options)) {
      // endpoints can be created ahead of time
      // or created directly by the ServiceEndpoint builder.
      Endpoint endpoint = Endpoint.builder()
          .name("AuthCallbackEndpoint")
          .subject("$SYS.REQ.USER.AUTH")
          .build();

      AuthCalloutHandler handler = new AuthCalloutHandler(nc);

      ServiceEndpoint serviceEndpoint = ServiceEndpoint.builder()
          .endpoint(endpoint)
          .handler(handler)
          .build();

      // Create the service from service endpoint.
      Service acService = new ServiceBuilder()
          .connection(nc)
          .name("AuthCallbackService")
          .version("0.0.1")
          .addServiceEndpoint(serviceEndpoint)
          .build();

      System.out.println("\n" + acService);

      // ----------------------------------------------------------------------------------------------------
      // Start the services
      // ----------------------------------------------------------------------------------------------------
      CompletableFuture<Boolean> serviceStoppedFuture = acService.startService();

      // plenty of time to finish running the main.sh example script
      Thread.sleep(20000);

      // A real service will do something like this
      // serviceStoppedFuture.get();
    }
    catch (Exception e) {
      e.printStackTrace();
    }
  }
}

