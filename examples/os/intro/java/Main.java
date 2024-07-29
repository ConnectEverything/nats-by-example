package example;

import io.nats.client.*;
import io.nats.client.api.*;
import io.nats.client.impl.ErrorListenerConsoleImpl;
import io.nats.client.support.Digester;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.security.NoSuchAlgorithmException;
import java.util.concurrent.ThreadLocalRandom;

public class Main {
  public static void main(String[] args) {
    String natsURL = System.getenv("NATS_URL");
    if (natsURL == null) {
      natsURL = "nats://127.0.0.1:4222";
    }

    Options options = Options.builder().server(natsURL).errorListener(new ObjectStoreIntroErrorListener()).build();

    try (Connection nc = Nats.connect(options)) {
      // The utility of the Object Store is that it is able to store
      // objects that are larger than the max message payload
      System.out.println("The server max payload is configured as " + nc.getServerInfo().getMaxPayload() + " bytes.");

      // ### Bucket basics
      // An object store (OS) bucket is created by specifying a bucket name.
      // Java returns a ObjectStoreStatus object upon creation
      ObjectStoreManagement osm = nc.objectStoreManagement();

      ObjectStoreConfiguration osc = ObjectStoreConfiguration.builder()
          .name("my-bucket")
          .description("bucket with my stuff in it")
          .storageType(StorageType.Memory)
          .compression(true)
          .build();

      // when you create a bucket, you get a status object in return.
      ObjectStoreStatus objectStoreStatus = osm.create(osc);

      // Retrieve the Object Store context by name once the bucket is created.
      ObjectStore os = nc.objectStore("my-bucket");

      System.out.println("Before put, the object store has " + objectStoreStatus.getSize() + " bytes of data and meta-data stored.");

      int bytes = 10_000_000;
      byte[] data = new byte[bytes];
      ThreadLocalRandom.current().nextBytes(data);
      ByteArrayInputStream in = new ByteArrayInputStream(data);

      // ### PUT
      // There are multiple ways to "put" an object into the store.
      // This examples shows putting an input stream like you might get from a file
      // It also shows how to customize how it's put into the store,
      // in this case in chunks of 32K instead of teh default 128K
      // Setting chunk size is important to consider for instance
      // if you know you are on a connection with limited bandwidth or
      // your server has been configured to only accept smaller messages.
      ObjectMeta objectMeta = ObjectMeta.builder("my-object")
          .description("My Object Description")
          .chunkSize(32 * 1024)
          .build();
      ObjectInfo oInfo = os.put(objectMeta, in);
      System.out.println("Object '" + oInfo.getObjectName() + "' ('" + oInfo.getDescription() + "')"
          + " was successfully put in bucket '" + oInfo.getBucket() + "'.");

      objectStoreStatus = os.getStatus();
      System.out.println("After put, the object store has " + objectStoreStatus.getSize() + " bytes of data and meta-data stored.");

      oInfo = os.getInfo("my-object");
      String digest = oInfo.getDigest();
      System.out.println("Object '" + oInfo.getObjectName()
          + "'\n  has " + oInfo.getSize() + " bytes"
          + "'\n  has a digest of '" + digest + "'"
          + "\n  and is stored in " + oInfo.getChunks() + " chunks (messages)."
      );

      // ### GET
      // When we "get" an object, we will need an output stream
      // to deliver the bytes. It could be a FileOutputStream to store
      // it directly to disk. In this case we will just put it to memory.
      ByteArrayOutputStream out = new ByteArrayOutputStream();

      // The get reads all the message chunks.
      os.get("my-object", out);
      byte[] outBytes = out.toByteArray();

      System.out.println("We received " + outBytes.length + " bytes.");

      // Manually check the bytes
      for (int x = 0; x < outBytes.length; x++) {
        if (data[x] != outBytes[x]) {
          System.out.println("Input should be exactly output.");
          return;
        }
      }
      System.out.println("We received exactly the same bytes we put in!");

      // Use a digester to check the bytes
      Digester d = new Digester();
      d.update(outBytes);
      String outDigest = d.getDigestEntry();
      System.out.println("The received bytes has a digest of: '" + outDigest + "'");

      // ### Watch the store
      // We can watch the store for changes or even just to see all the objects in the store
      ObjectStoreWatcher uoWatcher = new ObjectStoreIntroWatcher("UPDATES_ONLY");
      System.out.println("About to watch [UPDATES_ONLY]");
      os.watch(uoWatcher, ObjectStoreWatchOption.UPDATES_ONLY);

      byte[] simple = new byte[100_000];
      ThreadLocalRandom.current().nextBytes(simple);
      os.put("simple object", simple);
      System.out.println("Simple object has been put.");

      ObjectStoreWatcher ihWatcher = new ObjectStoreIntroWatcher("INCLUDE_HISTORY");
      System.out.println("About to watch [INCLUDE_HISTORY]");
      os.watch(ihWatcher, ObjectStoreWatchOption.INCLUDE_HISTORY);

      os.delete("simple object");
      os.delete("my-object");

      // Sleep this thread a little so the program has time
      // to receive all the messages before the program quits.
      Thread.sleep(500);

      objectStoreStatus = os.getStatus();
      System.out.println("After deletes, the object store has " + objectStoreStatus.getSize() + " bytes of data and meta-data stored.");

    } catch (InterruptedException | IOException | JetStreamApiException e) {
      e.printStackTrace();
    }
    catch (NoSuchAlgorithmException e) {
      // The NoSuchAlgorithmException comes from the Digester if the runtime does
      // not have the SHA-256 algorithm available, so should actually never happen.
      throw new RuntimeException(e);
    }
  }

  // ### ObjectStoreWatcher
  // This is just a simple implementation.
  static class ObjectStoreIntroWatcher implements ObjectStoreWatcher {
    String name;

    public ObjectStoreIntroWatcher(String name) {
      this.name = name;
    }

    @Override
    public void watch(ObjectInfo objectInfo) {
      System.out.println("Watcher [" + name + "] The object store has an object named: " + objectInfo.getObjectName());
    }

    @Override
    public void endOfData() {
    }
  }

  // ### Custom Error listener
  // Since it currently uses a push consumer under the covers
  // depending on the size of the object, the ability / speed for it to process the incoming
  // messages, there may be some flow control warnings. These are just warnings.
  // In custom error listeners you can turn of printing or logging of that warning.
  static class ObjectStoreIntroErrorListener extends ErrorListenerConsoleImpl {
    @Override
    public void flowControlProcessed(Connection conn, JetStreamSubscription sub, String id, FlowControlSource source) {
      // do nothing, just a warning
    }
  }
}
