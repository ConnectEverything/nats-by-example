package example;

import io.nats.client.*;
import io.nats.client.api.*;

import java.io.IOException;
import java.time.Duration;
import java.util.Iterator;
import java.util.List;
import java.util.concurrent.CompletableFuture;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    try (Connection conn = Nats.connect(natsURL)) {
      JetStreamManagement jsm = nc.jetStreamManagement();
      JetStream js = jsm.jetStream();

      // Create a stream with a few subjects
      jsm.addStream(StreamConfiguration.builder()
          .name("subjects")
          .subjects("plain", "greater.>", "star.*")
          .build());

      // ### GetStreamInfo with StreamInfoOptions
      // Get the subjects via the getStreamInfo call.
      // Since this is "state" there are no subjects in the state unless
      // there are messages in the subject.
      StreamInfo si = jsm.getStreamInfo("subjects", StreamInfoOptions.allSubjects());
      StreamState state = si.getStreamState();
      System.out.println("Before publishing any messages, there are 0 subjects: " + state.getSubjectCount());

      // Publish a message
      js.publish("plain", null);

      si = jsm.getStreamInfo("subjects", StreamInfoOptions.allSubjects());
      state = si.getStreamState();
      System.out.println("After publishing a message to a subject, it appears in state:");
      for (Subject s : state.getSubjects()) {
        System.out.println("  "  + s);
      }

      // Publish some more messages, this time against wildcard subjects
      js.publish("greater.A", null);
      js.publish("greater.A.B", null);
      js.publish("greater.A.B.C", null);
      js.publish("greater.B.B.B", null);
      js.publish("star.1", null);
      js.publish("star.2", null);

      si = jsm.getStreamInfo("subjects", StreamInfoOptions.allSubjects());
      state = si.getStreamState();
      System.out.println("Wildcard subjects show the actual subject, not the template.");
      for (Subject s : state.getSubjects()) {
        System.out.println("  "  + s);
      }

      // ### Subject Filtering
      // Instead of allSubjects, you can filter for a specific subject
      si = jsm.getStreamInfo("subjects", StreamInfoOptions.filterSubjects("greater.>"));
      state = si.getStreamState();
      System.out.println("Filtering the subject returns only matching entries ['greater.>']");
      for (Subject s : state.getSubjects()) {
        System.out.println("  "  + s);
      }

      si = jsm.getStreamInfo("subjects", StreamInfoOptions.filterSubjects("greater.A.>"));
      state = si.getStreamState();
      System.out.println("Filtering the subject returns only matching entries ['greater.A.>']");
      for (Subject s : state.getSubjects()) {
        System.out.println("  "  + s);
      }
    }
    catch (JetStreamApiException | IOException | InterruptedException e) {
      // * JetStreamApiException: the stream or consumer did not exist
      // * IOException: problem making the connection
      // * InterruptedException: thread interruption in the body of the example
      System.out.println(e);
    }
  }
}
