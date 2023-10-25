// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect } from "https://deno.land/x/nats@v1.17.0/src/mod.ts";
import { ServiceError } from "https://deno.land/x/nats@v1.17.0/nats-base-client/core.ts";

const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

const nc = await connect({
  servers: servers.split(","),
});

// ### Defining a service
//
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

// ### Adding endpoints
//
// For the minmax service we are going to reserve a root subject of
// `minmax` and group services in it. This will prevent polluting the
// subject space. You can add nested groups and mix with endpoints
// as your API requires.
const root = srv.addGroup("minmax");

// Now add an endpoint - the endpoint is a subscription. The
// endpoint will put together the name of the endpoint and any previous
// subjects together. In this case `max` is the endpoint, and it will
// be reachable via `minmax.max`
root.addEndpoint("max", {
  handler: (err, msg) => {
    if (err) {
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

// Alternately, the JavaScript client can use iterators to handle requests
// instead of the `handler` callback.
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
  srv.stop(err).finally(() => {});
});

// ### Sending requests
//
// Now let's make a couple requests into the service which uses the standard
// `nc.request` API. Note the subject hiearchy is composed of the group and
// the endpoint.
let r = await nc.request("minmax.max", JSON.stringify([-1, 2, 100, -2000]));
if (!ServiceError.isServiceError(r)) {
  console.log("max value", r.json());
} else {
  console.log(ServiceError.toServiceError(r));
}

r = await nc.request("minmax.min", JSON.stringify([-1, 2, 100, -2000]));
if (!ServiceError.isServiceError(r)) {
  console.log("min value", r.json());
} else {
  console.log(ServiceError.toServiceError(r));
}

// ### Service APIs
//
// Services defined by the micro framework expose a set of APIs that
// can be used to query various aspects of the service. This includes
// the `ping`, `info`, and `stats` APIs. The top-level subjects of these
// APIs are `$SRV.PING`, `$SRV.INFO`, and `$SRV.STATS`, respectively.
// The service `name` and `id` can be appended to the subject to narrow
// the scope of the query. For example, `$SRV.PING.minmax` will ping
// the `minmax` service. The `id` can be used to query a specific
// instance of a service, identified by the unique ID. For example
// `$SRV.PING.minmax.1` will ping the service instance with ID `1`.
const sc = nc.services.client();

// The `ping` API will ping each service that is online and will return
// basic information about the service(s) that respond.
let iter = await sc.ping();
console.log("results from PING");
for await (const si of iter) {
  console.log(si);
}

// The `info` API is a superset of the `ping` API and will return
// more information, such as the endpoints per service.
iter = await sc.info();
console.log("results from INFO");
for await (const i of iter) {
  console.log(i);
}

// Finally, the `stats` API returns a statistics report for each service
// broken down by endpoint.
iter = await sc.stats();
console.log("results from STATS");
for await (const si of iter) {
  console.log(si);
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

await srv.stop();
await nc.close();
