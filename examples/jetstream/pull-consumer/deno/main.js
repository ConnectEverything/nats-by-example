// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import {
  AckPolicy,
  connect,
  millis,
  nuid,
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

// make the stream/subjects unique
const subj = nuid.next();
const name = `EVENTS_${subj}`;
await jsm.streams.add({
  name,
  subjects: [`${subj}.>`],
});
// Publish a few messages for the example.
await Promise.all([
  js.publish(`${subj}.1`),
  js.publish(`${subj}.2`),
  js.publish(`${subj}.3`),
]);

// The new consumer API is a pull consumer
// Let's create an ephemeral consumer. An ephemeral consumer
// will be reaped by the server when inactive for some time
let ci = await jsm.consumers.add(name, { ack_policy: AckPolicy.None });
// by simply specifying the name of the stream
const c = await js.consumers.get(name, ci.name);
console.log(
  "ephemeral consumer will live until inactivity of ",
  millis((await c.info(true)).config.inactive_threshold),
  "millis",
);

// you can retrieve messages one at time with next():
let m = await c.next();
console.log(m.subject);

m = await c.next();
console.log(m.subject);

m = await c.next();
console.log(m.subject);

// Let's create another consumer, this time well use fetch
// we'll make this a durable
await jsm.consumers.add(name, {
  ack_policy: AckPolicy.Explicit,
  durable_name: "A",
});
// by simply specifying the name of the stream
const c2 = await js.consumers.get(name, "A");

let iter = await c2.fetch({ max_messages: 3 });
for await (const m of iter) {
  console.log(m.subject);
  m.ack();
}
// if you know you don't need to save the state of the consumer, you can
// delete it:
await c2.delete();

// Lastly we'll create another one but this time use consume
// this consumer will be an ordered consumer - this one is an ephemeral
// that guarantees that messages are delivered in order
// These have a special shortcut, we only need the name of the stream
// the underlying consumer is managed under the covers
const c3 = await js.consumers.get(name);

iter = await c3.consume({ max_messages: 3 });
for await (const m of iter) {
  console.log(m.subject);
  // if we don't break, consume would keep waiting for messages
  // we know when we have seen all messages when no more are pending
  if (m.info.pending === 0) {
    break;
  }
}

await nc.drain();
