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

  // ----------------------------------------------------------------------------------------------------
  // The AuthCalloutHandler is the service implementation that handles the Auth Callout from the server.
  // This is the real work for this example.
  // ----------------------------------------------------------------------------------------------------
  static class AuthCalloutHandler implements ServiceMessageHandler {
    static String ISSUER_NSEED = "SAANDLKMXL6CUS3CP52WIXBEDN6YJ545GDKC65U5JZPPV6WH6ESWUA6YAI";

    static final Map<String, AuthCalloutUser> NATS_USERS;

    static final NKey USER_SIGNING_NKEY;
    static final String USER_SIGNING_PUBLIC_KEY;

    static {
      try {
        USER_SIGNING_NKEY = NKey.fromSeed(ISSUER_NSEED.toCharArray());
        USER_SIGNING_PUBLIC_KEY = new String(USER_SIGNING_NKEY.getPublicKey());
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
      Permission pa = new Permission().allow("alice.>");
      NATS_USERS.put("alice", new AuthCalloutUser().userPass("alice").account("APP")); // .sub(pa));
      Permission pb = new Permission().allow("bob.>");
      ResponsePermission r = new ResponsePermission().max(1);
      NATS_USERS.put("bob", new AuthCalloutUser().userPass("bob").account("APP").pub(pb).sub(pb).resp(r));
      NATS_USERS.put("pub", new AuthCalloutUser().userPass("pub").account("APP"));
      NATS_USERS.put("reset", new AuthCalloutUser().userPass("reset").account("APP"));
    }

    Connection nc;

    public AuthCalloutHandler(Connection nc) {
      this.nc = nc;
    }

    @Override
    public void onMessage(ServiceMessage smsg) {
      System.out.println("[HANDLER] Received Message");
      System.out.println("[HANDLER] Subject       : " + smsg.getSubject());
      System.out.println("[HANDLER] Headers       : " + headersToString(smsg.getHeaders()));

      try {
        // Convert the message data into a Claim
        Claim claim = new Claim(getClaimBody(smsg.getData()));
        System.out.println("[HANDLER] Claim-Request : " + claim.toJson());

        // The Claim should contain an Authorization Request
        AuthorizationRequest ar = claim.authorizationRequest;
        if (ar == null) {
          System.err.println("Invalid Authorization Request Claim");
          return;
        }
        printJson("[HANDLER] Auth Request  : ", ar.toJson(), "server_id", "user_nkey", "client_info", "connect_opts", "client_tls", "request_nonce");

        // Check if the user exists. In the example we are just checking the NATS_USERS map
        // but in the real world, this will be back by something specific to your organization.
        AuthCalloutUser acUser = NATS_USERS.get(ar.connectOpts.user);
        if (acUser == null) {
          // The user was not found, respond with an error
          // * smsg is the original message, it contains a reply-to
          // * ar is the AuthorizationRequest
          // * there is no userJwt in this response
          // * make some text string and the error will be logged
          respond(smsg, ar, null, "User Not Found: " + ar.connectOpts.user);
          return;
        }
        if (!acUser.pass.equals(ar.connectOpts.pass)) {
          // The user password didn't match, respond with an error
          // * smsg is the original message, it contains a reply-to
          // * ar is the AuthorizationRequest
          // * there is no userJwt in this response
          // * make some text string and the error will be logged
          respond(smsg, ar, null, "Password does not match: " + acUser.pass + " != " + ar.connectOpts.pass);
          return;
        }

        // The user is found, start a UserClaim to give back to the server
        // copy the publish and subscribe rights and response permissions
        UserClaim uc = new UserClaim()
            .pub(acUser.pub)
            .sub(acUser.sub)
            .resp(acUser.resp);

        // After creating the UserClaim, we must issue a JWT based on it.
        String userJwt = new ClaimIssuer()
            .aud(acUser.account)
            .name(ar.connectOpts.user)
            .iss(USER_SIGNING_PUBLIC_KEY)
            .sub(ar.userNkey)
            .nats(uc)
            .issueJwt(USER_SIGNING_NKEY);

        // The auth was successful
        // Call the method to respond to the server
        // * smsg is the original message, it contains a reply-to
        // * ar is the AuthorizationRequest
        // * userJwt is the JWT string
        // * there is no error in this case
        respond(smsg, ar, userJwt, null);
      }
      catch (Exception e) {
        //noinspection CallToPrintStackTrace
        e.printStackTrace();
      }
    }

    static boolean flag = true;

    private void respond(ServiceMessage smsg,
                         AuthorizationRequest ar,
                         String userJwt,
                         String error) throws GeneralSecurityException, IOException {

      AuthorizationResponse response = new AuthorizationResponse()
          .jwt(userJwt)
          .error(error);

      if (userJwt != null) {
        printJson("[HANDLER] Auth Resp JWT : ", getClaimBody(userJwt), "name", "nats");
      }
      else {
        System.out.println("[HANDLER] Auth Resp ERR : " + response.toJson());
      }

      String jwt = new ClaimIssuer()
          .aud(ar.serverId.id)
          .iss(USER_SIGNING_PUBLIC_KEY)
          .sub(ar.userNkey)
          .nats(response)
          .issueJwt(USER_SIGNING_NKEY);

      System.out.println("[HANDLER] Claim-Response: " + getClaimBody(jwt));
      smsg.respond(nc, jwt);
    }
  }

  // ----------------------------------------------------------------------------------------------------
  // The AuthCalloutUser is just a simple data structure.
  // ----------------------------------------------------------------------------------------------------
  static class AuthCalloutUser {
    public String user;
    public String pass;
    public String account;
    public Permission pub;
    public Permission sub;
    public ResponsePermission resp;

    public AuthCalloutUser userPass(String userPass) {
      this.user = userPass;
      this.pass = userPass;
      return this;
    }

    public AuthCalloutUser user(String user) {
      this.user = user;
      return this;
    }

    public AuthCalloutUser pass(String pass) {
      this.pass = pass;
      return this;
    }

    public AuthCalloutUser account(String account) {
      this.account = account;
      return this;
    }

    public AuthCalloutUser pub(Permission pub) {
      this.pub = pub;
      return this;
    }

    public AuthCalloutUser sub(Permission sub) {
      this.sub = sub;
      return this;
    }

    public AuthCalloutUser resp(ResponsePermission resp) {
      this.resp = resp;
      return this;
    }
  }

  // ----------------------------------------------------------------------------------------------------
  // Example support functions
  // ----------------------------------------------------------------------------------------------------
  static final String SPACER = "                                                            ";
  static void printJson(String label, String json, String... splits) {
    if (splits != null && splits.length > 0) {
      String indent = SPACER.substring(0, label.length());
      boolean first = true;
      for (String split : splits) {
        int at = json.indexOf("\"" + split + "\"");
        if (at > 0) {
          if (first) {
            first = false;
            System.out.println(label + json.substring(0, at));
          }
          else {
            System.out.println(indent + json.substring(0, at));
          }
          json = json.substring(at);
        }
      }
      System.out.println(indent + json);
    }
    else {
      System.out.println(label + json);
    }
  }

  static String headersToString(Headers h) {
    if (h == null || h.isEmpty()) {
      return "None";
    }

    boolean notFirst = false;
    StringBuilder sb = new StringBuilder("[");
    for (String key : h.keySet()) {
      if (notFirst) {
        sb.append(',');
      }
      else {
        notFirst = true;
      }
      sb.append(key).append("=").append(h.get(key));
    }
    return sb.append(']').toString();
  }
}

