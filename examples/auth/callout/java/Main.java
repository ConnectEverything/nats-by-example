package example;

import io.nats.client.*;
import io.nats.client.impl.Headers;
import io.nats.jwt.*;
import io.nats.service.*;

import java.io.IOException;
import java.security.GeneralSecurityException;
import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;

import static io.nats.jwt.Utils.getClaimBody;

public class Main {

  public static void main(String[] args) throws Exception {
    Options options = new Options.Builder()
        .server("nats://localhost:4222")
        .errorListener(new ErrorListener() {
        })
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

      Thread.sleep(20_000); // plenty of time to finish running the main.sh example script

      // a real app would likely use this future
      // serviceStoppedFuture.get();
    }
    catch (Exception e) {
      e.printStackTrace();
    }
  }
}

