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
  static String ISSUER_NSEED = "SAANDLKMXL6CUS3CP52WIXBEDN6YJ545GDKC65U5JZPPV6WH6ESWUA6YAI";

  static final Map<String, AuthCalloutUser> NATS_USERS;

  static final NKey USER_SIGNING_KEY;
  static final String PUB_USER_SIGNING_KEY;

  static {
    try {
      USER_SIGNING_KEY = NKey.fromSeed(ISSUER_NSEED.toCharArray());
      PUB_USER_SIGNING_KEY = new String(USER_SIGNING_KEY.getPublicKey());
    }
    catch (Exception e) {
      throw new RuntimeException(e);
    }

    // This sets up a map of users to simulate a back end auth system
    //   sys/sys, SYS
    //   alice/alice, APP
    //   bob/bob, APP, pub allow "bob.>", sub allow "bob.>", response max 1
    NATS_USERS = new HashMap<>();
    NATS_USERS.put("sys", new AuthCalloutUser().userPass("sys").account("SYS"));
    NATS_USERS.put("alice", new AuthCalloutUser().userPass("alice").account("APP"));
    Permission p = new Permission().allow("bob.>");
    ResponsePermission r = new ResponsePermission().max(1);
    NATS_USERS.put("bob", new AuthCalloutUser().userPass("bob").account("APP").pub(p).sub(p).resp(r));
  }

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

