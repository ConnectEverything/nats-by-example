import {
  AckPolicy,
  nuid,
  connect
} from "https://deno.land/x/nats@v1.28.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL")?.split(",");

const nc = await connect({ servers });

const jsm = await nc.jetstreamManager();
const js = nc.jetstream();

// Create the stream
const subj = nuid.next();
const name = `EVENTS_${subj}`;
await jsm.streams.add({
  name,
  subjects: [subj]
});

// Publish a couple messages so we can look at the state
await js.publish(subj)
await js.publish(subj)

// Consume a message with 2 different consumers
// The first consumer will (regular) ack without confirmation
// The second consumer will ackSync which confirms that ack was handled.

// Consumer 1 will use ack()
const ci1 = await jsm.consumers.add(name, {
  name: "consumer1",
  filter_subject: subj,
  ack_policy: AckPolicy.Explicit
});
console.log("Consumer 1");
console.log("  Start");
console.log(`    pending messages: ${ci1.num_pending}`);
console.log(`    messages with ack pending: ${ci1.num_ack_pending}`);

const consumer1 = await js.consumers.get(name, "consumer1");

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
const ci2 = await jsm.consumers.add(name, {
  name: "consumer2",
  filter_subject: subj,
  ack_policy: AckPolicy.Explicit
});
console.log("Consumer 2");
console.log("  Start");
console.log(`    pending messages: ${ci2.num_pending}`);
console.log(`    messages with ack pending: ${ci2.num_ack_pending}`);

const consumer2 = await js.consumers.get(name, "consumer2");

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
