// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect } from "https://deno.land/x/nats@v1.16.0/src/mod.ts";

// Get the passed NATS_URL or fallback to the default. This can be
// a comma-separated string.
const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await connect({
  servers: servers.split(","),
});


// To publish a message, simply provide the _subject_ of the message
// and encode the message payload. NATS subjects are hierarchical using
// periods as token delimiters. `greet` and `joe` are two distinct tokens.
nc.publish("greet.bob", "hello");

// Now we are going to create a subscription and utilize a wildcard on
// the second token. The effect is that this subscription shows _interest_
// in all messages published to a subject with two tokens where the first
// is `greet`.
let sub = nc.subscribe("greet.*", {max: 3});
const done = (async () => {
  for await (const msg of sub) {
    console.log(`${msg.string()} on subject ${msg.subject}`);
  }
})()

// Let's publish three more messages which will result in the messages
// being forwarded to the local subscription we have.
nc.publish("greet.joe", "hello");
nc.publish("greet.pam", "hello");
nc.publish("greet.sue", "hello");

// This will wait until the above async subscription handler finishes
// processing the three messages. Note that the first message to
// `greet.bob` was not printed. This is because the subscription was
// created _after_ the publish. Core NATS provides at-most-once quality
// of service (QoS) for active subscriptions.
await done;

// Finally we drain the connection which waits for any pending
// messages (published or in a subscription) to be flushed.
await nc.drain();
