import {
  AckPolicy,
  StorageType,
  connect
} from "https://deno.land/x/nats@v1.28.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL")?.split(",");

const nc = await connect({ servers });

// create a stream with a random name with some messages and a consumer
const stream = "confirmAckStream";
const subject = "confirmAckSubject";

const jsm = await nc.jetstreamManager();
const js = nc.jetstream();

// Create a stream
// (remove the stream first so we have a clean starting point)
try {
  await jsm.streams.delete(stream);
} catch (err) {
  if (err.code != 404) {
    console.error(err.message);
  }
}

await jsm.streams.add({
  name: stream,
  subjects: [subject],
  storage: StorageType.Memory,
});

// Publish a couple messages so we can look at the state
await js.publish(subject)
await js.publish(subject)

// Consume a message with 2 different consumers
// The first consumer will (regular) ack without confirmation
// The second consumer will ackSync which confirms that ack was handled.

// Consumer 1 will use ack()
const ci1 = await jsm.consumers.add(stream, {
  name: "consumer1",
  filter_subject: subject,
  ack_policy: AckPolicy.Explicit
});
console.log("Consumer 1");
console.log("  Start");
console.log(`    pending messages: ${ci1.num_pending}`);
console.log(`    messages with ack pending: ${ci1.num_ack_pending}`);

const consumer1 = await js.consumers.get(stream, "consumer1");

try {
  const m = await consumer1.next();
  if (m) {
    let ci1= await consumer1.info(false);
    console.log("  After received but before ack");
    console.log(`    pending messages: ${ci1.num_pending}`);
    console.log(`    messages with ack pending: ${ci1.num_ack_pending}`);

    m.ack()
    ci1 = await consumer1.info(false);
    console.log("  After ack");
    console.log(`    pending messages: ${ci1.num_pending}`);
    console.log(`    messages with ack pending: ${ci1.num_ack_pending}`);
  }
} catch (err) {
  console.log(`consume failed: ${err.message}`);
}


// Consumer 2 will use ackAck()
const ci2 = await jsm.consumers.add(stream, {
  name: "consumer2",
  filter_subject: subject,
  ack_policy: AckPolicy.Explicit
});
console.log("Consumer 2");
console.log("  Start");
console.log(`    pending messages: ${ci2.num_pending}`);
console.log(`    messages with ack pending: ${ci2.num_ack_pending}`);

const consumer2 = await js.consumers.get(stream, "consumer2");

try {
  const m = await consumer2.next();
  if (m) {
    let ci2= await consumer2.info(false);
    console.log("  After received but before ack");
    console.log(`    pending messages: ${ci2.num_pending}`);
    console.log(`    messages with ack pending: ${ci2.num_ack_pending}`);

    await m.ackAck()
    ci2 = await consumer2.info(false);
    console.log("  After ack");
    console.log(`    pending messages: ${ci2.num_pending}`);
    console.log(`    messages with ack pending: ${ci2.num_ack_pending}`);
  }
} catch (err) {
  console.log(`consume failed: ${err.message}`);
}

await nc.close();
