// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import {
  AckPolicy,
  connect,
  millis,
  nuid,
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

// ### Creating the stream
// Define the stream configuration, specifying `WorkQueuePolicy` for
// retention, and create the stream.
const jsm = await js.jetstreamManager();
await jsm.streams.add({
  name: "EVENTS",
  retention: RetentionPolicy.Workqueue,
  subjects: ["events.>"],
});
console.log("created the stream");

// ### Queue messages
// Publish a few messages.
await Promise.all([
  js.publish("events.us.page_loaded"),
  js.publish("events.eu.mouse_clicked"),
  js.publish("events.us.input_focused"),
]);
console.log("published 3 messages");

// Checking the stream info, we see three messages have been queued.
console.log("# Stream info without consumers");
console.log((await jsm.streams.info("EVENTS")).state);

// ### Adding a consumer
// Now let's add a consumer and publish a few more messages.
// See /examples/jetstream/pull-consumer/deno
await jsm.consumers.add("EVENTS", {
  durable_name: "worker",
  ack_policy: AckPolicy.Explicit,
});

// Get a pull consumer
const c = await js.consumers.get("EVENTS", "worker");
// Fetch and ack the queued messages
const iter = await c.fetch({ max_messages: 3 });
for await (const m of iter) {
  m.ack();
}

// Checking the stream info again, we will notice no messages
// are available as we have ack'ed them.
console.log("# Stream info with one consumer");
console.log((await jsm.streams.info("EVENTS")).state);

// ### Exclusive non-filtered consumer
// As noted in the description above, work-queue streams can only have
// at most one consumer with interest on a subject at any given time.
// Since the pull consumer above is not filtered, if we try to create
// another one, it will fail.
console.log("# Create an overlapping consumer");
await jsm.consumers.add("EVENTS", {
  durable_name: "worker2",
  ack_policy: AckPolicy.Explicit,
}).catch((err) => {
  console.log(err.message);
});

// However if we delete the first consumer we can add a new one
await c.delete();
await jsm.consumers.add("EVENTS", {
  durable_name: "worker2",
  ack_policy: AckPolicy.Explicit,
});
console.log("created the new consumer");
await jsm.consumers.delete("EVENTS", "worker2");

// ### Multiple filtered consumers
// To create multiple consumers, a subject filter needs to be applied.
// For this example, we could scope each consumer to the geo that the
// event was published from, in this case `us` or `eu`.
console.log("# Create non-overlapping consumers");

await jsm.consumers.add("EVENTS", {
  durable_name: "worker-us",
  ack_policy: AckPolicy.Explicit,
  filter_subject: "events.us.>",
});

await jsm.consumers.add("EVENTS", {
  durable_name: "worker-eu",
  ack_policy: AckPolicy.Explicit,
  filter_subject: "events.eu.>",
});

async function process(name, count) {
  const usc = await js.consumers.get("EVENTS", name);
  const iter = await usc.fetch({ max_messages: count });
  for await (const m of iter) {
    console.log(`${name} got: ${m.subject}`);
    m.ack();
  }
}

const a = process("worker-us");
const b = process("worker-eu");

await Promise.all([
  js.publish("events.eu.mouse_clicked"),
  js.publish("events.us.page_loaded"),
  js.publish("events.us.input_focused"),
  js.publish("events.eu.page_loaded"),
]);
console.log("published 4 messages");

await Promise.all([a, b]);
await nc.close();
