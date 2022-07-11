import * as nats from "https://deno.land/x/nats@v1.7.1/src/mod.ts";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string.
const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await nats.connect({
  servers: servers.split(","),
});

// NATS message payloads are byte arrays, so we need to have a codec
// to serialize and deserialize payloads in order to work with them.
// Another built-in codec is JSONCodec or you can implement your own.
const sc = nats.StringCodec();

// To publish a message, simply provide the _subject_ of the message
// and encode the message payload. NATS subjects are hierarchical using
// periods as token delimiters. `greet` and `joe` are two distinct tokens.
await nc.publish("greet.joe", sc.encode("hello"));

// Now we are going to create a subscription and utilize a wildcard on
// the second token. The effect is that this subscription show _interest_
// in all messages published to a subject with two tokens where the first
// is `greet`.
const sub = nc.subscribe("greet.*");

// We are going to try any get a message which will time out since the
// previously published message is now gone. Core NATS provides an
// at-most-once guarantee. In this context, this means if a subscription

// TODO: How can I only fetch one message with a timeout.. this (expectedly) just hangs.
for await (const msg of sub) {}

// Let's publish three more messages which will result in the messages
// being forwarded to the local subscription we have.
await nc.publish("greet.joe", sc.encode("hello"));
await nc.publish("greet.pam", sc.encode("hello"));
await nc.publish("greet.sue", sc.encode("hello"));

// Now we can receive these messages. Notice that each subject is different
// but they match the subscription wildcard.
// TODO: likewise.. a more elegant way to only get three messages?
let count = 0;
for await (const msg of sub) {
  count++
  console.log(`${sc.decode(msg.data)} from subject ${msg.subject}`);
  if (count == 3) {
    break;
  }
}

// Process any pending messages and then unsubscribe.
await sub.unsubscribe();

// Drain the connection which waits for any pending messages (published
// or in a subscription) to be flushed.
await nc.drain();
