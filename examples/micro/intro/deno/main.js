// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect } from "https://deno.land/x/nats@v1.17.0/src/mod.ts";
import { ServiceError } from "https://deno.land/x/nats@v1.17.0/nats-base-client/core.ts";

const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

const nc = await connect({
  servers: servers.split(","),
});

// ### Microservices basics
// Define the service. Note that at this point no service endpoints
// have been added, but the service is already discoverable!
const srv = await nc.services.add({
  name: `minmax`,
  version: `0.0.1`,
  description: `returns the max/min number in a request`,
});

// The service can do something special when it stops.
// The error argument will be set if the stop was due to an error
srv.stopped.then((err) => {
  console.log(`service stopped ${err ? err.message : ""}`);
});

// For the minmax service we are going to reserve a root subject of
// `minmax` and group services in it. This will prevent polluting the
// subject space. You can add nested groups and mix with endpoints
// as your API requires
const root = srv.addGroup("minmax");
// Now add an endpoint - the endpoint is a subscription. The
// endpoint will put together the name of the endpoint and any previous
// subjects together. In this case `max` is the endpoint, and it will
// be reachable via `minmax.max`
root.addEndpoint("max", {
  handler: (err, msg) => {
    if (err) {
      // We got a permissions error, we'll simply report it and stop
      srv.stop(err).finally(() => {});
    }
    // The payload is an encoded JSON array, the decode function
    // simply performs some small checks and validations and
    // returns an array that is sorted or null. If null, the payload
    // was invalid, and we respond with an error to the requester.
    const values = decode(msg);
    if (values) {
      msg?.respond(JSON.stringify({ max: values[values.length - 1] }));
    }
  },
  metadata: {
    schema: "input a JSON serialized JSON array, output the largest value",
  },
});

// The javascript client can also use iterators to handle requests
const min = root.addEndpoint("min", {
  metadata: {
    schema: "input a JSON serialized JSON array, output the smallest value",
  },
});
(async () => {
  for await (const msg of min) {
    const values = decode(msg);
    if (values) {
      msg?.respond(JSON.stringify({ max: values[0] }));
    }
  }
})().catch((err) => {
  // the iterator stopped due to an error
  srv.stop(err).finally(() => {});
});

// The services framework is fairly straight forward. Now let's make
// a couple requests into the service
let r = await nc.request("minmax.max", JSON.stringify([-1, 2, 100, -2000]));
// Clients can check if there was an error processing the request
if (!ServiceError.isServiceError(r)) {
  console.log("max value", r.json());
} else {
  console.log(ServiceError.toServiceError(r));
}
// And the for the `min` portion...
r = await nc.request("minmax.min", JSON.stringify([-1, 2, 100, -2000]));
if (!ServiceError.isServiceError(r)) {
  console.log("min value", r.json());
} else {
  console.log(ServiceError.toServiceError(r));
}

// The javascript clients provide a simple client for interacting
// with services created with the micro framework. The subjects
// used are simply `SRV.<PING|STATS|INFO>`
// if a service name is specified:
// `SRV.<PING|STATS|INFO>.<service>`
// if a service name and id:
// `SRV.<PING|STATS|INFO>.<service>.<id>`

const sc = nc.services.client();
// The `ping()` request returns service infos. Our services will
// return an info. The monitoring and discovery APIs can target
// specific service names or particular service IDs, for now we
// simply want to see all services built with micro that are out there.
let iter = await sc.ping();
console.log("results from PING");
for await (const si of iter) {
  console.log(si);
}
// the stats API returns a status report of for each endpoint
iter = await sc.stats();
console.log("results from STATS");
for await (const si of iter) {
  console.log(si);
}
// returns info about the service and each endpoint
console.log("results from INFO");
iter = await sc.info();
for await (const i of iter) {
  console.log(i);
}

function decode(m) {
  try {
    const a = m?.json();
    if (!Array.isArray(a)) {
      m?.respondError(400, "invalid payload");
      return null;
    }
    if (a?.length === 0) {
      m?.respondError(400, "no values provided");
      return null;
    }
    a.sort();
    return a;
  } catch (err) {
    m?.respondError(400, `unable to decode: ${err.message}`);
    return null;
  }
}

// cleanup
await srv.stop();
await nc.close();
