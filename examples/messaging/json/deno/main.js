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

// Create a subscription that receives two messages. One message will
// contain a valid serialized payload and the other will not.
const sub = await nc.subscribe("foo", { max: 2 });
(async () => {
  for await (const m of sub) {
    try {
      // payload of the message `m.data` is an Uint8Array
      // m.json() parses the JSON payload - this can fail.
      // m.string() parses the payload as a string
      console.log(m.json());
    } catch (err) {
      console.log(`err: ${err.message}: '${m.string()}'`);
    }
  }
})();

// publish the messages
nc.publish("foo", JSON.stringify({ foo: "bar", bar: 27 }));
nc.publish("foo", "not json");

await nc.drain();
