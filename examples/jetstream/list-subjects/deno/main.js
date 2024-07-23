import {
    StorageType,
    connect
} from "https://deno.land/x/nats@v1.28.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL")?.split(",");

const nc = await connect({ servers });

const jsm = await nc.jetstreamManager();
const js = nc.jetstream();

// Create a stream
// (remove the stream first so we have a clean starting point)
try {
    await jsm.streams.delete("subjects");
} catch (err) {
    if (err.code != 404) {
        console.error(err.message);
    }
}

// Create a stream with a few subjects
let si = await jsm.streams.add({
    name: "subjects",
    subjects: ["plain", "greater.>", "star.*"],
    storage: StorageType.Memory,
});

// ### Get stream info with StreamInfoRequestOptions
// Get the subjects via the streams.info call.
// Since this is "state" there are no subjects in the state unless
// there are messages in the subject.
si = await jsm.streams.info("subjects", {subjects_filter: ">"});

const count = si.state.subjects
    ? Object.getOwnPropertyNames(si.state.subjects).length
    : 0;
console.log(`Before publishing any messages, there are 0 subjects: ${count}`)

// console.log(`Before publishing any messages, there are 0 subjects: ${si.state.num_subjects}`)

// Publish a message
await js.publish("plain")

si = await jsm.streams.info("subjects", {subjects_filter: ">"});
console.log("After publishing a message to a subject, it appears in state:")

if (si.state.subjects) {
    let subjects = {};
    subjects = Object.assign(subjects, si.state.subjects);
    for (const key of Object.keys(subjects)) {
        console.log(`  subject '${key}' has ${si.state.subjects[key]} message(s)`)
    }
}

// Publish some more messages, this time against wildcard subjects
await js.publish("greater.A", "gtA-1");
await js.publish("greater.A", "gtA-2");
await js.publish("greater.A.B", "gtAB-1");
await js.publish("greater.A.B", "gtAB-2");
await js.publish("greater.A.B.C", "gtABC");
await js.publish("greater.B.B.B", "gtBBB");
await js.publish("star.1", "star1-1");
await js.publish("star.1", "star1-2");
await js.publish("star.2", "star2");

// Get all subjects
si = await jsm.streams.info("subjects", {subjects_filter: ">"});
console.log("Wildcard subjects show the actual subject, not the template.");

if (si.state.subjects) {
    let subjects = {};
    subjects = Object.assign(subjects, si.state.subjects);
    for (const key of Object.keys(subjects)) {
        console.log(`  subject '${key}' has ${si.state.subjects[key]} message(s)`)
    }
}

// ### Subject Filtering
// Instead of allSubjects, you can filter for a specific subject
si = await jsm.streams.info("subjects", {subjects_filter: "greater.>"});
console.log("Filtering the subject returns only matching entries ['greater.>']");

if (si.state.subjects) {
    let subjects = {};
    subjects = Object.assign(subjects, si.state.subjects);
    for (const key of Object.keys(subjects)) {
        console.log(`  subject '${key}' has ${si.state.subjects[key]} message(s)`)
    }
}

si = await jsm.streams.info("subjects", {subjects_filter: "greater.A.>"});
console.log("Filtering the subject returns only matching entries ['greater.A.>']");

if (si.state.subjects) {
    let subjects = {};
    subjects = Object.assign(subjects, si.state.subjects);
    for (const key of Object.keys(subjects)) {
        console.log(`  subject '${key}' has ${si.state.subjects[key]} message(s)`)
    }
}

await nc.close();
