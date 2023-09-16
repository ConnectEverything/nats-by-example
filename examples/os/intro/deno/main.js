// import the library - in node.js `import {connect, etc} from "nats";`
// or if not doing a module, `const {connect, etc} = require("nats");`
import { connect } from "https://deno.land/x/nats@v1.16.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL") || "nats://localhost:4222";

const nc = await connect({
  servers: servers.split(","),
});

// ### Object store basics
// An object-store (OS) bucket is created by specifying a bucket name.
const js = nc.jetstream();

// Here we try to access a store called "configs", if it doesn't exist
// the API will create it:
const os = await js.views.os("configs");

// You can get information on the object store by getting its info:
let status = await os.status();
console.log(`the object store has ${status.size} bytes`);

// 10MiB
const bytes = 10000000;
let data = new Uint8Array(bytes);

// The JavaScript client has a simple API to `putBlob()` and `getBlob()`
// Let's add an entry to the object store
let info = await os.putBlob({ name: "a", description: "large data" }, data);
console.log(
  `added entry ${info.name} (${info.size} bytes)- '${info.description}'`,
);

// Entries in an object store are made from a "metadata" that describes the object
// And the payload. This allows you to store information about the significance of the
// entry separate from the raw data.
// You can update the metadata directly
await os.update("a", { description: "still large data" });

// we expect this store to only contain one entry
// You can list its contents:
let entries = await os.list();
console.log(`the object store contains ${entries.length} entries`);

// Now lets retrieve the item, if the entry doesn't exist it will return null
data = await os.getBlob("a");
console.log(`data has ${data?.length} bytes`);

// if you are reading carefully, the payload we stored and read, is 10MiB in
// size, but the server by default has a max payload of about 1MiB.
// How can this be possible? - Ah that is what ObjectStore is about
// it automatically splits data into multiple chunks and stores it.
// The client automatically split into multiple messages and stored in the
// stream. On read, it read multiple messages and put it together.
console.log(`client has a max payload of ${nc.info?.max_payload} bytes`);

// You can watch an object store for changes:
const iter = await os.watch({ includeHistory: false });
(async () => {
  for await (const s of iter) {
    console.log(
      `${s.bucket} changed - ${s.name} ${
        s.deleted ? "was deleted" : "was updated"
      }`,
    );
  }
})();

// To delete an entry:
await os.delete("a");

// Because the client may be working with large assets, ObjectStore
// normally presents a "Stream" based API. In the case of JavaScript
// you load data from a `ReadableStream`.
// Here's a function that takes the same large byte array and turns it
// into a Readable stream:
function readableStreamFrom(data) {
  return new ReadableStream({
    pull(controller) {
      controller.enqueue(data);
      controller.close();
    },
  });
}

// Normally depending on the platform
// you will get readable streams when reading files or fetching data.
// For purposes of this example we'll use this function
let rs = readableStreamFrom(data);

info = await os.put(
  { name: "b", description: "set with a readable stream" },
  rs,
);
console.log(
  `added entry ${info.name} (${info.size} bytes)- '${info.description}'`,
);

// To read the entry:
const result = await os.get("b");
// you can read the info on the object:
console.log(`${result.info.name} has ${result.info.size} bytes`);

// to get the payload, you get a readable stream from data
const reader = result.data.getReader();
while (true) {
  const { done, value } = await reader.read();
  if (done) {
    break;
  }
  // else stage the data
  console.log(`read ${value.length} bytes`);
}

// the readable stream api, can return an error as you read
// the data, so when you think you are done reading, you must
// check if there was an error
const err = await result.error;
if (err) {
  // if here, discard whatever you were staging
  console.log(`there was an error reading the ReadableStream: ${err.message}`);
} else {
  // if no error, the data is good - you can save it etc.
  console.log("done reading");
}
// You can delete the object store by simply calling `destroy()`.
// Note that after calling destroy, the data is gone forever.
await os.destroy();
console.log(`deleted object store`);

await nc.close();
