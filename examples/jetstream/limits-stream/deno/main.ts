import {
  connect,
  nanos,
  StringCodec,
  StreamConfig,
  StreamInfo,
  PubAck,
} from "https://deno.land/x/nats@v1.7.1/src/mod.ts";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string.
const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await connect({
  servers: servers.split(","),
});

// NATS message payloads are byte arrays, so we need to have a codec
// to serialize and deserialize payloads in order to work with them.
// Another built-in codec is JSONCodec or you can implement your own.
const sc = StringCodec();

// Access the JetStream manager which provides the methods for managing
// streams and consumers.
const jsm = await nc.jetstreamManager();

// Declare the initial stream config. A stream can bind one or more
// subjects that are not overlapping with other streams. By default,
// a stream will have one replica and use file storage.
const cfg: StreamConfig = {
  name: "EVENTS",
  subjects: ["events.>"],
};

// Add/create the stream.
await jsm.streams.add(cfg)
console.log("created the stream")

// Access the JetStream client for publishing and subscribing to streams.
const js = nc.jetstream();

// Publish a series of messages and wait for each one to be completed.
await js.publish("events.page_loaded");
await js.publish("events.mouse_clicked");
await js.publish("events.mouse_clicked");
await js.publish("events.page_loaded");
await js.publish("events.mouse_clicked");
await js.publish("events.input_focused");
console.log("published 6 messages");

const events = [
  "events.input_changed",
  "events.input_blurred",
  "events.key_pressed",
  "events.input_focused",
  "events.input_changed",
  "events.input_blurred",
];

// Map over the events which returns a set of promises as a batch.
// Then wait until all of them are done before proceeding.
const batch: Promise<PubAck>[] = events.map((e) => js.publish(e));
await Promise.all(batch);
console.log("published another 6 messages");

// Get the stream state to show 12 messages exist.
let info: StreamInfo = await jsm.streams.info(cfg.name);
console.log(info.state);

// Let's update the stream config and set the max messages to 10. This
// can be done with a partial config passed to the `update` method.
await jsm.streams.update(cfg.name, {max_msgs: 10});

// Once applied, we can check out the stream state again to see that the
// first two messages were truncated.
info = await jsm.streams.info(cfg.name);
console.log(info.state);

// Updating the stream config again, we can set the max bytes of the stream
// overall.
await jsm.streams.update(cfg.name, {max_bytes: 300});

// This will prune the messages down some more...
info = await jsm.streams.info(cfg.name);
console.log(info.state);

// Finally the last limit of max_age can be applied. Note the age
// is in nanoseconds, so 1000 milliseconds (1 second) converted to nanos.
await jsm.streams.update(cfg.name, {max_age: nanos(1000)});

// Sleep for a second to ensure the message age in the stream has lapsed.
await new Promise(r => setTimeout(r, 1000));

info = await jsm.streams.info(cfg.name);
console.log(info.state);

// Finally we drain the connection which waits for any pending
// messages (published or in a subscription) to be flushed.
await nc.drain();
