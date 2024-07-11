package example;

import io.nats.client.*;
import io.nats.client.api.*;

import java.io.IOException;
import java.util.Map;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    try (Connection nc = Nats.connect(natsURL)) {
      JetStreamManagement jsm = nc.jetStreamManagement();
      JetStream js = jsm.jetStream();

      // Delete the stream, so we always have a fresh start for the example
      // don't care if this, errors in this example, it will if the stream exists.
      try {
        jsm.deleteStream("subjects");
      }
      catch (Exception ignore) {}

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
        System.out.println("  subject '" + s.getName() + "' has " + s.getCount() + " message(s)");
      }

      // Publish some more messages, this time against wildcard subjects
      js.publish("greater.A", "gtA-1".getBytes());
      js.publish("greater.A", "gtA-2".getBytes());
      js.publish("greater.A.B", "gtAB-1".getBytes());
      js.publish("greater.A.B", "gtAB-2".getBytes());
      js.publish("greater.A.B.C", "gtABC".getBytes());
      js.publish("greater.B.B.B", "gtBBB".getBytes());
      js.publish("star.1", "star1-1".getBytes());
      js.publish("star.1", "star1-2".getBytes());
      js.publish("star.2", "star2".getBytes());

      // Get all subjects, but get the subjects as a map
      si = jsm.getStreamInfo("subjects", StreamInfoOptions.allSubjects());
      state = si.getStreamState();
      System.out.println("Wildcard subjects show the actual subject, not the template.");
      for (Map.Entry<String, Long> entry : state.getSubjectMap().entrySet()) {
        System.out.println("  subject '" + entry.getKey() + "' has " + entry.getValue() + " message(s)");
      }

      // ### Subject Filtering
      // Instead of allSubjects, you can filter for a specific subject
      si = jsm.getStreamInfo("subjects", StreamInfoOptions.filterSubjects("greater.>"));
      state = si.getStreamState();
      System.out.println("Filtering the subject returns only matching entries ['greater.>']");
      for (Subject s : state.getSubjects()) {
        System.out.println("  subject '" + s.getName() + "' has " + s.getCount() + " message(s)");
      }

      si = jsm.getStreamInfo("subjects", StreamInfoOptions.filterSubjects("greater.A.>"));
      state = si.getStreamState();
      System.out.println("Filtering the subject returns only matching entries ['greater.A.>']");
      for (Subject s : state.getSubjects()) {
        System.out.println("  subject '" + s.getName() + "' has " + s.getCount() + " message(s)");
      }
    } catch (InterruptedException | IOException | JetStreamApiException e) {
      e.printStackTrace();
    }
  }
}
