// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect, Empty } from "https://deno.land/x/nats@v1.16.0/src/mod.ts";


const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

// Create a client connection to an available NATS server.
const nc = await connect({
  servers: servers.split(","),
});

// In addition to vanilla publish-request, NATS supports request-reply
// interactions as well. Under the covers, this is just an optimized
// pair of publish-subscribe operations.
// The _request handler_ is just a subscription that _responds_ to a message
// sent to it. This kind of subscription is called a _service_.
const sub = nc.subscribe("greet.*", {
  callback: (err, msg) => {
    if (err) {
      console.log("subscription error", err.message);
      return;
    }
    // Parse out the second token in the subject (everything after greet.)
    // and use it as part of the response message.
    const name = msg.subject.substring(6);
    msg.respond(`hello, ${name}`);
  },
});

// Now we can use the built-in `request` method to do the service request.
// A payload is optional, and we skip setting it right now. In addition,
// you can specify an optional timeout, but we'll use the default for now.
let rep = await nc.request("greet.joe");
console.log(rep.string());

// here put a payload
rep = await nc.request("greet.sue", "hello!");
console.log(rep.string());

// and here we set a timeout (with an empty payload), and a timeout
// of 3 seconds - specified as milliseconds
rep = await nc.request("greet.bob", Empty, {timeout: 3000});
console.log(rep.string());

// What happens if the service is _unavailable_? We can simulate this by
// unsubscribing our handler from above. Now if we make a request, we will
// expect an error.
sub.unsubscribe();

nc.request("greet.joe")
  .catch((err) => {
    console.log(
      "request failed with: ",
      err.code === "503" ? "timeout" : err.message,
    );
  });


// Close the connection
await nc.drain();
