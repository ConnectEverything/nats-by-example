import { AckPolicy, connect, consumerOpts } from "nats";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string. If not defined, it will default to localhost:4222
const servers = process.env.NATS_URL?.split(",");
console.log(servers);

// Create a client connection to an available NATS server.
const nc = await (connect({ servers }));

// Create a JetStream Manager context - this context has API we can use to
// create streams and consumers.
const jsm = await nc.jetstreamManager();
// and create a stream
await jsm.streams.add({
  name: "EVENTS",
  subjects: ["events.>"],
});

// We also create a JetStream context that provides a mechanism
// for publishing data to JetStream and to create "subscriptions"
// on a consumer:
const js = nc.jetstream();
// Here we add 20 messages to a stream
const proms = Array.from({ length: 20 })
  .map((_v, idx) => {
    return js.publish(`events.${idx}`);
  });
await Promise.all(proms);

// Processing messages using Legacy JetStream is fairly cumbersome
// and because the JetStream subscription could create or update consumers
// under the covers, it leads to some complications. Specially if you had
// more than a single process that examined a stream. You could get unintended
// consumer updates, etc.

// # Legacy Processing a Stream
//
// Using the legacy API, the easiest way to continuously receive messages
// is to use push consumers.
// The `subscribe()` API call was intended for these "push" consumers. These
// look very natural to NATS users.
let opts = consumerOpts()
  .deliverTo("eventprocessing")
  .ackExplicit()
  .manualAck();

const pushSub = await js.subscribe("events.>", opts);
for await (const m of pushSub) {
  console.log(`legacy push subscriber received ${m.subject}`);
  m.ack();
  // we can check if the stream has more messages
  // by checking the number of pending messages, and break
  // if we are done.
  if (m.info.pending === 0) {
    // breaking from the iterator will stop the subscription
    // after a while the consumer will be deleted if not done
    break;
  }
}
// destroy() deletes the consumer!!! - this is not really necessary
// as the server will auto destroy an ephemeral consumer. If you know
// the consumer is not going to be needed, then destroying it will
// help with resource management
await pushSub.destroy();

// The above was actually quite easy - however for real streams that
// could contain huge number of messages, it required that you set up
// other options, and if you didn't you could run into issues such as
// slow consumers, or having a consumer that cannot be horizontally scaled.

// To prevent those sort of issues, the legacy API also provided a
// `pullSubscribe()`, which effectively avoided the issues of the push
// (where the client can be overwhelmed with messages). However, this
// API is hard to use:
opts = consumerOpts()
  .ackExplicit()
  .manualAck();

// pullSubscribe() would create a subscription to process messages
// received from the stream, but would require a `pull()` to trigger
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

// To get messages flowing, you needed to call `pull()` on
// the subscription
pullSub.pull({ batch: 15, no_wait: true });
// and also do so at some interval to keep it going - however
// there was no coordination between the processing of the
// messages and the triggering of the pulls:
const timer = setInterval(() => {
  pullSub.pull({ batch: 15, no_wait: true });
}, 1000);

await done;
clearInterval(timer);

// New API for consuming messages on a stream, is way more ergonomic
// and easier to use. Internally it uses a "pull consumer", handles the
// requesting of messages under the hood. As an user, you simply
// can specify how many messages you want to buffer, and the library will
// do its best to keep that up for you.
//
// The new API doesn't automatically create or update consumers. This
// is something that the JetStreamManager API does rather well. Instead,
// you simply use JetStreamManager to create your consumer:
await jsm.consumers.add("EVENTS", {
  name: "my-ephemeral",
  ack_policy: AckPolicy.Explicit,
});

// To process messages, you retrieve the consumer, by specifying the name
// of the stream and the name of the consumer
const consumerA = await js.consumers.get("EVENTS", "my-ephemeral");

// With a consumer in hand, you can now retrieve messages - in different ways.
// Firstly, we'll discuss consume, this is analogous to the push consumer example
// above:
const messages = await consumerA.consume();
for await (const m of messages) {
  console.log(`consume received ${m.subject}`);
  m.ack();
  if (m.info.pending === 0) {
    break;
  }
}
// if you wanted to preempt delete the consumer you can - however
// this is something you should do only if you know you are not
// going to need that consumer.
await consumerA.delete();

// Let's create a new consumer, this time a durable
await jsm.consumers.add("EVENTS", {
  durable_name: "my-durable",
  ack_policy: AckPolicy.Explicit,
});

// The different ways of getting messages from the consumers are there
// to help you align the buffering requirements of your application with
// what the client is doing.

// The legacy API provided a way of retrieving a single message:
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

// With the new JetStream API we can do the same:
const consumerB = await js.consumers.get("EVENTS", "my-durable");
consumerB.next()
  .then((m) => {
    // the API is more ergonomic, if no messages it will be null
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

// Finally some clients will want to manage the rate at which they receive
// messages more explicitly. Legacy JetStream provided the fetch() call which
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
