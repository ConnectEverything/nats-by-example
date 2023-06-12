// import the library, note that if you are running in node:
// `import { AckPolicy, connect, consumerOpts } from "nats";`
import {
  AckPolicy,
  connect,
  consumerOpts,
} from "https://deno.land/x/nats@v1.14.0/src/mod.ts";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string. If not defined, it will default to localhost:4222
// in node, you can access the environment:
// `const servers = process.env.NATS_URL?.split(",");`
const servers = Deno.env.get("NATS_URL")?.split(",");

// Create a client connection to an available NATS server.
const nc = await (connect({ servers }));

// Resource creation has not changed. To create a stream and consumers,
// create a JetStream Manager context - this context has API you can use to
// create those resources
const jsm = await nc.jetstreamManager();
await jsm.streams.add({
  name: "EVENTS",
  subjects: ["events.>"],
});

// To add messages to the stream, create a JetStream context, and
// publish data to the stream
const js = nc.jetstream();
const proms = Array.from({ length: 20 })
  .map((_v, idx) => {
    return js.publish(`events.${idx}`);
  });
await Promise.all(proms);

// ### Processing Messages
// Now lets compare and contrast the new and legacy ways of processing
// messages.

// #### Legacy Push Subscribe
//
// Previously, the easiest way to continuously receive messages
// was to use _push_ consumer. The `subscribe()` API call was
// intended for these _push_ consumers. These looked very natural
// to NATS users.


// the legacy `subscribe()` variants relied on consumer options
// being provided. These options defined the consumer to use.
// if the consumer didn't exist, it would be created, if it did,
// and the options were different, the consumer would be updated
let opts = consumerOpts()
  .deliverTo("eventprocessing")
  .ackExplicit()
  .manualAck();

// the subscribe call automatically creates a consumer that matches the specified
// options, and returns an async iterator with the messages from the stream.
// If no messages are available, the loop will wait.
const pushSub = await js.subscribe("events.>", opts);
for await (const m of pushSub) {
  console.log(`legacy push subscriber received ${m.subject}`);
  m.ack();
  // you can check if the stream currently has more messages
  // by checking the number of pending messages, and break
  // if you are done - typically your code will simply wait until
  // new messages become available
  if (m.info.pending === 0) {
    // breaking from the iterator will stop the subscription
    break;
  }
}
// `destroy()` deletes the consumer! - this is not really necessary on ephemeral
// consumers, since the server will destroy them after some specified inactivity.
// If you know the consumer is not going to be needed, then destroying it will
// help with resource management. Durable consumers are not deleted, will
// consume resources forever if not managed.
await pushSub.destroy();

// #### Legacy Pull Subscription
//
// The above is quite easy - however for streams that
// contain huge number of messages, it required that you set up
// other options, and if you didn't you could run into issues such as
// _slow consumers_, or have a consumer that cannot be horizontally scaled.
//
// To prevent those issues, the legacy API also provided a
// `pullSubscribe()`, which effectively avoided the issues of _push_
// by enabling the client to request the number of messages it wanted
// to process:
opts = consumerOpts()
  .ackExplicit()
  .manualAck();

// `pullSubscribe()` would create a subscription to process messages
// received from the stream, but required a `pull()` to trigger
// a request on the server to yield messages
const pullSub = await js.pullSubscribe("events.>", opts);
const done = (async () => {
  for await (const m of pullSub) {
    console.log(`legacy pull subscriber received ${m.subject}`);
    m.ack();
    if (m.info.pending === 0) {
      return;
    }
  }
})();

// To get messages flowing, you called `pull()` on
// the subscription to start
pullSub.pull({ batch: 15, no_wait: true });
// and also do so at some interval to keep messages flowing.
// Unfortunately, there no coordination between the processing
// of the messages and the triggering of the pulls was provided.
const timer = setInterval(() => {
  pullSub.pull({ batch: 15, no_wait: true });
}, 1000);

await done;
clearInterval(timer);

// ### New JetStream Processing API
//
// The new API doesn't automatically create or update consumers. This
// is something that the JetStreamManager API does rather well. Instead,
// you simply use JetStreamManager to create your consumer:
await jsm.consumers.add("EVENTS", {
  name: "my-ephemeral",
  ack_policy: AckPolicy.Explicit,
});

// To process messages, you retrieve the consumer by specifying
// the stream name and consumer names. If the consumer doesn't exist
// this call will reject. Note that only _pull consumers_ are supported.
// If your existing consumer is a push consumer, you will have to recreate
// it as a pull consumer (not specifying a `deliver_subject` option on a
// consumer configuration, nor specifying `deliverTo()` as an option):
const consumerA = await js.consumers.get("EVENTS", "my-ephemeral");

// With a consumer in hand, you can now retrieve messages - in different ways.
// The different ways of getting messages from the consumers are there
// to help you align the buffering requirements of your application with
// what the client is doing.

// #### Consuming Messages
//
// Firstly, we'll discuss consume, this is analogous to the push consumer example
// above, where the consumer will yield messages from the stream to match any
// buffering options specified on the call. The defaults are safe, however you
// can ask for as many messages as you will be able to process within your ack window.
// As you consume messages, the library will retrieve more messages for you.
// Yes, under the hood this is actually a pull consumer, but that actually works smartly
// for you.
const messages = await consumerA.consume({ max_messages: 5000 });
for await (const m of messages) {
  console.log(`consume received ${m.subject}`);
  m.ack();
  if (m.info.pending === 0) {
    break;
  }
}

// if you wanted to preempt delete the consumer you can - however
// this is something you should do only if you know you are not
// going to need that consumer to resume processing.
await consumerA.delete();

// Let's create a new consumer, this time a durable
await jsm.consumers.add("EVENTS", {
  durable_name: "my-durable",
  ack_policy: AckPolicy.Explicit,
});

// #### Processing Single Messages
//
// Some clients such as services typically to worry about processing a single message
// at a time. The idea being, instead of optimizing a client to pull many messages
// for processing, you can horizontally scale the number of process that work on
// just one message.
//
// #### Legacy Pull
//
// The legacy API provided `pull()` as a way of retrieving a single message:
const m = await js.pull("EVENTS", "my-durable")
  .catch((err) => {
    // possibly no messages found
    console.log(err.message);
    return null;
  });

if (m === null) {
  console.log("legacy pull got no messages");
} else {
  console.log(`jetstream legacy pull: ${m.subject}`);
  m.ack();
}

// #### Get
//
// With the new JetStream API we can do the same, but it is now called `get()`:
const consumerB = await js.consumers.get("EVENTS", "my-durable");
consumerB.next()
  .then((m) => {
    // the API is more ergonomic, if no messages it will be `null`
    if (m === null) {
      console.log("consumer next - no messages available");
    } else {
      console.log(`consumer next - ${m.subject}`);
      m.ack();
    }
  })
  .catch((err) => {
    // here we have some error
    console.error(err.message);
  });

// Processing a Small Batch of Messages
//
// Finally some clients will want to manage the rate at which they receive
// messages more explicitly.
//
// Legacy JetStream provided the `fetch()` API which
// returned one or more messages in a single request:
let iter = await js.fetch("EVENTS", "my-durable", { batch: 3, expires: 5000 });
for await (const m of iter) {
  console.log(`legacy fetch: ${m.subject}`);
  m.ack();
}

// The new API also provides the same facilities - notice we already
// retrieved the consumer as `consumer`. The batch property, is now called
// `max_messages`:
iter = await consumerB.fetch({ max_messages: 3, expires: 5000 });
for await (const m of iter) {
  console.log(`consumer fetch: ${m.subject}`);
  m.ack();
}

await nc.drain();
