// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import {
  AckPolicy,
  connect,
  RetentionPolicy,
} from "https://deno.land/x/nats@v1.16.0/src/mod.ts";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string.
const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await connect({
  servers: servers.split(","),
});

// access JetStream
const js = nc.jetstream();
// CRUD operations in jetstream are performed by the JetStreamManager:
const jsm = await js.jetstreamManager();

// ### Creating the stream
// Define the stream configuration, specifying `RetentionPolicy.Interest` for retention, and
// create the stream.
await jsm.streams.add({
  name: "EVENTS",
  retention: RetentionPolicy.Interest,
  subjects: ["events.>"],
});
console.log("created the stream");

// To demonstrate the base case behavior of the stream without any consumers, we
// will publish a few messages to the stream.
await js.publish("events.page_loaded");
await js.publish("events.mouse_clicked");
let ack = await js.publish("events.input_focused");
console.log("published 3 messages");

// We confirm that all three messages were published and the last message sequence
// is 3.
console.log("last message seq: ", ack.seq);

// Checking out the stream info, notice how zero messages are present in
// the stream, but the `last_seq` is 3 which matches the last ack'ed
// publish sequence above. Also notice that the `first_seq` is one greater
// which behaves as a sentinel value indicating the stream is empty. This
// sequence has not been assigned to a message yet, but can be interpreted
// as _no messages available_ in this context.
console.log("# Stream info without any consumers");
console.log((await jsm.streams.info("EVENTS")).state);

// ### Adding a consumer
// Now let's add a consumer and publish a few more messages
await jsm.consumers.add("EVENTS", {
  durable_name: "processor-1",
  ack_policy: AckPolicy.Explicit,
});

await js.publish("events.mouse_clicked");
await js.publish("events.input_focused");

// If we inspect the stream info again, we will notice a few differences.
// It shows two messages (which we expect) and the first and last sequences
// corresponding to the two messages we just published. We also see that
// the `consumer_count` is now one.
console.log("# Stream info with one consumer");
console.log((await jsm.streams.info("EVENTS")).state);

// Now that the consumer is there and showing _interest_ in the messages, we know they
// will remain until we process the messages. Let's create a Consumer and process the
// messages - note we are accessing JetStream, not JetStreamManager here
const c = await js.consumers.get("EVENTS", "processor-1");
let iter = await c.fetch({ max_messages: 2, expires: 1000 });
await (async () => {
  for await (const m of iter) {
    m.ack();
  }
})();

// What do we expect in the stream? No messages and the `first_seq` has been set to
// the _next_ sequence number like in the base case.
console.log("# Stream info with one consumer and acked messages");
console.log((await jsm.streams.info("EVENTS")).state);

// ### Two or more consumers
// Since each consumer represents a separate _view_ over a stream, we would expect
// that if messages were processed by one consumer, but not the other, the messages
// would be retained. This is indeed the case.
await jsm.consumers.add("EVENTS", {
  durable_name: "processor-2",
  ack_policy: AckPolicy.Explicit,
});

await js.publish("events.input_focused");
await js.publish("events.mouse_clicked");

// Here we get the second consumer `processor-2`, followed by a fetch and ack. There are
// two observations to make here. First the fetched messages are the latest two messages
// that were published just above and not any prior messages since these were already
// deleted from the stream. This should be apparent now, but this reinforces that a _late_
// consumer cannot retroactively show interest.
// The second point is that the stream info shows that the latest two messages are still
// present in the stream. This is also expected since the first consumer had not yet
// processed them.
const c2 = await js.consumers.get("EVENTS", "processor-2");
iter = await c2.fetch({ max_messages: 2, expires: 1000 });
await (async () => {
  for await (const m of iter) {
    console.log(`msg stream seq: ${m.info.streamSequence}`);
    m.ack();
  }
})();

console.log(
  "# Stream info with two consumers, but only one set of acked messages",
);
console.log((await jsm.streams.info("EVENTS")).state);

// Fetching and ack'ing from the first consumer subscription will result in the messages
// being deleted.
iter = await c.fetch({ max_messages: 2, expires: 1000 });
await (async () => {
  for await (const m of iter) {
    m.ack();
  }
})();

console.log("# Stream info with two consumers having both acked");
console.log((await jsm.streams.info("EVENTS")).state);

// A final callout is that _interest_ respects the `FilterSubject` on a consumer.
// For example, if a consumer defines a filter only for `events.mouse_clicked` events
// then it won't be considered _interested_ in events such as `events.input_focused`.
await jsm.consumers.add("EVENTS", {
  durable_name: "processor-3",
  ack_policy: AckPolicy.Explicit,
  filter_subject: "events.mouse_clicked",
});

await js.publish("events.input_focused");

// Retrieve and term (also works) and ack from the first consumers that _do_ have interest.
let m = await c.next({ expires: 1000 });
m.term();

m = await c2.next({ expires: 1000 });
m.ack();

console.log("# Stream info with three consumers with interest from two");
console.log((await jsm.streams.info("EVENTS")).state);

await nc.drain();
