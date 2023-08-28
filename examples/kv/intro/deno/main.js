// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect } from "https://deno.land/x/nats@v1.16.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

const nc = await connect({
  servers: servers.split(","),
});

// ### Bucket basics
// A key-value (KV) bucket is created by specifying a bucket name.
const js = nc.jetstream();
const kv = await js.views.kv("profiles");

// As one would expect, the `KeyValue` interface provides the
// standard `Put` and `Get` methods. However, unlike most KV
// stores, a revision number of the entry is tracked.
await kv.put("sue.color", "blue");
let entry = await kv.get("sue.color");
console.log(`${entry?.key} @ ${entry?.revision} -> ${entry?.string()}`);

await kv.put("sue.color", "green");
entry = await kv.get("sue.color");
console.log(`${entry?.key} @ ${entry?.revision} -> ${entry?.string()}`);

// A revision number is useful when you need to enforce [optimistic
// concurrency control][occ] on a specific key-value entry. In short,
// if there are multiple actors attempting to put a new value for a
// key concurrently, we want to prevent the "last writer wins" behavior
// which is non-deterministic. To guard against this, we can use the
// `kv.Update` method and specify the expected revision. Only if this
// matches on the server, will the value be updated.
// [occ]: https://en.wikipedia.org/wiki/Optimistic_concurrency_control
await kv.update("sue.color", "red", 1)
  .then(() => {
    console.error("adding at version 1 should have failed!");
  })
  .catch((err) => {
    // we expect an error:
    console.log(err.message);
  });
entry = await kv.get("sue.color");
console.log(`${entry?.key} @ ${entry?.revision} -> ${entry?.string()}`);

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
const status = await kv.status();
console.log("KV stream name", status.streamInfo.config.name);

// Since it is a normal stream, we can create a consumer and
// fetch messages.
// If we look at the subject, we will notice that first token is a
// special reserved prefix, the second token is the bucket name, and
// remaining suffix is the actually key. The bucket name is inherently
// a namespace for all keys and thus there is no concern for conflict
// across buckets. This is different from what we need to do for a stream
// which is to bind a set of _public_ subjects to a stream.
const c = await js.consumers.get("KV_profiles");
let m = await c.next();
console.log(`${m.subject} @ ${m.info.streamSequence} -> '${m.string()}'`);

await kv.put("sue.color", "yellow");
m = await c.next();
console.log(`${m.subject} @ ${m.info.streamSequence} -> '${m.string()}'`);

// Unsurprisingly, we get the new updated value as a message.
// Since it's KV interface, we should be able to delete a key as well.
// Does this result in a new message?
await kv.delete("sue.color");
m = await c.next();
console.log(`${m.subject} @ ${m.info.streamSequence} -> '${m.string()}'`);

// 🤔 That is useful to get a message that something happened to that key,
// and that this is considered a new revision.
// However, how do we know if the new value was set to be `nil` or the key
// was deleted?
// To differentiate, delete-based messages contain a header. Notice the `KV-Operation: DEL`
// header.
console.log(`kv operation: ${m.headers.get("KV-Operation")}`);

// ### Watching for changes
// Although one could subscribe to the stream directly, it is more convenient
// to use a `KeyWatcher` which provides a deliberate API and types for tracking
// changes over time. Notice that we can use a wildcard which we will come back to..

let history = true;
const iter = await kv.watch({
  key: "sue.*",
  initializedFn: () => {
    history = false;
  },
});
(async () => {
  for await (const e of iter) {
    // Values marked with History are existing values - the watcher by default
    // shows the last value for all the keys in the KV
    console.log(
      `${
        history ? "History" : "Updated"
      } ${e.key} @ ${e.revision} -> ${e.string()}`,
    );
  }
})();

// To finish this short intro, since we know that keys are subjects under the covers, if we
// put another key, we can observe the change through the watcher. One other detail to call out
// is notice the revision for this *new* key is not `1`. It relies on the underlying stream's
// message sequence number to indicate the _revision_. The guarantee being that it is always
// monotonically increasing, but numbers will be shared across keys (like subjects) rather
// than sequence numbers relative to each key.
await kv.put("sue.food", "pizza");

await nc.close();
