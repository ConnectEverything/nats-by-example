package example;

import io.nats.client.*;
import io.nats.client.api.*;
import io.nats.client.impl.Headers;

import java.io.IOException;
import java.util.List;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    try (Connection nc = Nats.connect(natsURL)) {
      // ### What is a Service?
      //
      // A "service" consists of one or more endpoints.
      // An endpoint can be part of a group of endpoints or by itself.

      // ### Defining a Group
      //
      // In this example, the services will be part of a group.
      // The group name will be the prefix for the subject of the request.
      // Alternatively you could manually specify the group's subject
      // Here we create the group.
      Group serviceGroup = new Group("minmax");

      // ### Defining Endpoints
      // For each endpoint we give it a name. Like group,
      // you could manually specify the endpoint's subject.
      // In this example we are adding the endpoint to the group we defined
      // and are providing a ServiceMessageHandler implementation
      ServiceEndpoint min = ServiceEndpoint.builder()
          .endpointName("min")
          .group(serviceGroup)
          .handler(msg -> minRequestHandler(conn, msg))
          .build();

      ServiceEndpoint max = ServiceEndpoint.builder()
          .endpointName("max")
          .group(serviceGroup)
          .handler(msg -> maxRequestHandler(conn, msg))
          .build();

      // ### Defining the Service
      //
      // The Service definition requires a name and version, description is optional.
      // The name must be a simple name consisting of the characters A-Z, a-z, 0-9, dash (-) or underscore (_).
      // Add the endpoints that were created. Give the service a connection to run on.
      // A unique id is created for the service to identify it from different instances of the service.
      Service service = Service.builder()
          .name("minmax")
          .version("0.0.1")
          .description("Returns the min/max number in a request")
          .addServiceEndpoint(min)
          .addServiceEndpoint(max)
          .connection(conn)
          .build();

      System.out.println("Created Service: " + service.getName() + " with the id: " + service.getId());

      // ### Running the Service
      //
      // To run the service we call service.startService().
      // Uou can have a future that returns when service.stop() is called.
      CompletableFuture<Boolean> serviceStopFuture = service.startService();

      // For the example we use a simple string for the input and output
      // but in the real world it will be some sort of formatted data such as json.
      // The input and output is sent as the data payload of the NATS message.
      byte[] input = "-1,2,100,-2000".getBytes();

      // To "call" a service, we simply make a request to the proper endpoint with
      // data that it expects. Notice how the group name is prepended to the endpoint name.
      CompletableFuture<Message> minResponse = conn.request("minmax.min", input);
      CompletableFuture<Message> maxResponse = conn.request("minmax.max", input);

      Message minMessage = minResponse.get(1, TimeUnit.SECONDS);
      System.out.println("Min value is: " + new String(minMessage.getData()));

      Message maxMessage = maxResponse.get(1, TimeUnit.SECONDS);
      System.out.println("Max value is: " + new String(maxMessage.getData()));

      // The statistics being managed by micro should now reflect the call made
      // to each endpoint, and we didn't have to write any code to manage that.
      EndpointStats esMin = service.getEndpointStats(min.getName());
      System.out.println("The min service received " + esMin.getNumRequests() + " request(s).");

      EndpointStats esMax = service.getEndpointStats(max.getName());
      System.out.println("The max service received " + esMax.getNumRequests() + " request(s).");
    }
    catch (IOException | InterruptedException | ExecutionException | TimeoutException e) {
      // * IOException: problem making the connection
      // * InterruptedException: thread interruption in the body of the example
      // * ExecutionException: something went wrong in the request
      // * TimeoutException: the request took longer than the timeout specified
      System.err.println(e);
    }
  }
}
