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
      // ### Bucket basics
      // A key-value (KV) bucket is created by specifying a bucket name.
      // Java returns a KeyValueStatus object upon creation
      KeyValueManagement kvm = nc.keyValueManagement();

      KeyValueConfiguration kvc = KeyValueConfiguration.builder()
          .name("profiles")
          .build();

      KeyValueStatus keyValueStatus = kvm.create(kvc);

      // Retrieve the Key Value context once the bucket is created.
      KeyValue kv = nc.keyValue("profiles");

      // As one would expect, the `KeyValue` interface provides the
      // standard `Put` and `Get` methods. However, unlike most KV
      // stores, a revision number of the entry is tracked.
      kv.put("sue.color", "blue".getBytes());
      KeyValueEntry entry = kv.get("sue.color");
      System.out.printf("%s %d -> %s\n", entry.getKey(), entry.getRevision(), entry.getValueAsString());

      kv.put("sue.color", "green");
      entry = kv.get("sue.color");
      System.out.printf("%s %d -> %s\n", entry.getKey(), entry.getRevision(), entry.getValueAsString());

      // A revision number is useful when you need to enforce [optimistic
      // concurrency control][occ] on a specific key-value entry. In short,
      // if there are multiple actors attempting to put a new value for a
      // key concurrently, we want to prevent the "last writer wins" behavior
      // which is non-deterministic. To guard against this, we can use the
      // `kv.Update` method and specify the expected revision. Only if this
      // matches on the server, will the value be updated.
      // [occ]: https://en.wikipedia.org/wiki/Optimistic_concurrency_control
      try {
        kv.update("sue.color", "red".getBytes(), 1);
      }
      catch (JetStreamApiException e) {
        System.out.println(e);
      }

      long lastRevision = entry.getRevision();
      kv.update("sue.color", "red".getBytes(), lastRevision);
      entry = kv.get("sue.color");
      System.out.printf("%s %d -> %s\n", entry.getKey(), entry.getRevision(), entry.getValueAsString());

      // ### Stream abstraction
      // Before moving on, it is important to understand that a KV bucket is
      // light abstraction over a standard stream. This is by design since it
      // enables some powerful features which we will observe in a minute.
      //
      // **How exactly is a KV bucket modeled as a stream?**
      // When one is created, internally, a stream is created using the `KV_`
      // prefix as convention. Appropriate stream configuration are used that
      // are optimized for the KV access patterns, so you can ignore the
      // details.
      JetStreamManagement jsm = nc.jetStreamManagement();

      List<String> streamNames = jsm.getStreamNames();
      System.out.println(streamNames);

      // Since it is a normal stream, we can create a consumer and
      // fetch messages.
      // If we look at the subject, we will notice that first token is a
      // special reserved prefix, the second token is the bucket name, and
      // remaining suffix is the actualy key. The bucket name is inherently
      // a namespace for all keys and thus there is no concern for conflict
      // across buckets. This is different from what we need to do for a stream
      // which is to bind a set of _public_ subjects to a stream.
      JetStream js = nc.jetStream();

      PushSubscribeOptions pso = PushSubscribeOptions.builder()
          .stream("KV_profiles").build();
      JetStreamSubscription sub = js.subscribe(">", pso);

      Message m = sub.nextMessage(100);
      System.out.printf("%s %d -> %s\n", m.getSubject(), m.metaData().streamSequence(), new String(m.getData()));

      // Let's put a new value for this key and see what we get from the subscription.
      kv.put("sue.color", "yellow".getBytes());
      m = sub.nextMessage(100);
      System.out.printf("%s %d -> %s\n", m.getSubject(), m.metaData().streamSequence(), new String(m.getData()));

      // Unsurprisingly, we get the new updated value as a message.
      // Since it's a KV interface, we should be able to delete a key as well.
      // Does this result in a new message?
      kv.delete("sue.color");
      m = sub.nextMessage(100);
      System.out.printf("%s %d -> %s\n", m.getSubject(), m.metaData().streamSequence(), new String(m.getData()));

      // 🤔 That is useful to get a message that something happened to that key,
      // and that this is considered a new revision.
      // However, how do we know if the new value was set to be `nil` or the key
      // was deleted?
      // To differentiate, delete-based messages contain a header. Notice the `KV-Operation: DEL`
      // header.
      System.out.println("Headers:");
      Headers headers = m.getHeaders();
      for (String key : headers.keySet()) {
        System.out.printf("  %s:%s\n", key, headers.getFirst(key));
      }

      // ### Watching for changes
      // Although one could subscribe to the stream directly, it is more convenient
      // to use a `KeyValueWatcher` which provides a deliberate API and types for tracking
      // changes over time.
      KeyValueWatcher watcher = new KeyValueWatcher() {
        @Override
        public void watch(KeyValueEntry entry) {
          System.out.printf("Watcher: %s %d -> %s\n", entry.getKey(), entry.getRevision(), entry.getValueAsString());
        }

        // The end od data signal can be useful to known when the watcher has _caught up_ with the current updates before
        // tracking the new ones.
        @Override
        public void endOfData() {
          System.out.println("Watcher: Received End Of Data Signal");
        }
      };

      // Notice that we can use a wildcard for watching keys.
      kv.watch("sue.*", watcher, KeyValueWatchOption.UPDATES_ONLY);

      // Even though we deleted the key, of course we can put a new value.
      // In Java, there are a variety of `Put` signatures also, so here just put a string
      kv.put("sue.color", "purple");

      // To finish this short intro, since we know that keys are subjects under the covers, if we
      // put another key, we can observe the change through the watcher. One other detail to call out
      // is notice the revision for this *new* key is not `1`. It relies on the underlying stream's
      // message sequence number to indicate the _revision_. The guarantee being that it is always
      // monotonically increasing, but numbers will be shared across keys (like subjects) rather
      // than sequence numbers relative to each key.
      kv.put("sue.food", "pizza");

      // Sleep this thread a little so the program has time
      // to receive all the messages before the program quits.
      Thread.sleep(500);
    } catch (InterruptedException | IOException | JetStreamApiException e) {
      e.printStackTrace();
    }
  }
}
