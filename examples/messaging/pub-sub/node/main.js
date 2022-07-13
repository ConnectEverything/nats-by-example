import {connect, StringCodec} from "nats";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string.
const servers = process.env.NATS_URL || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await connect({
  servers: servers.split(","),
});

// NATS message payloads are byte arrays, so we need to have a codec
// to serialize and deserialize payloads in order to work with them.
// Another built-in codec is JSONCodec or you can implement your own.
const sc = StringCodec();

// To publish a message, simply provide the _subject_ of the message
// and encode the message payload. NATS subjects are hierarchical using
// periods as token delimiters. `greet` and `joe` are two distinct tokens.
nc.publish("greet.bob", sc.encode("hello"));

// Now we are going to create a subscription and utilize a wildcard on
// the second token. The effect is that this subscription shows _interest_
// in all messages published to a subject with two tokens where the first
// is `greet`.
let sub = nc.subscribe("greet.*", {max: 3});
const done = (async () => {
  for await (const msg of sub) {
    console.log(`${sc.decode(msg.data)} on subject ${msg.subject}`);
  }
})()

// Let's publish three more messages which will result in the messages
// being forwarded to the local subscription we have.
nc.publish("greet.joe", sc.encode("hello"));
nc.publish("greet.pam", sc.encode("hello"));
nc.publish("greet.sue", sc.encode("hello"));

// This will wait until the above async subscription handler finishes
// processing the three messages. Note that the first message to
// `greet.bob` was not printed. This is because the subscription was
// created _after_ the publish. Core NATS provides at-most-once quality
// of service (QoS) for active subscriptions.
await done;

// Finally we drain the connection which waits for any pending
// messages (published or in a subscription) to be flushed.
await nc.drain();
