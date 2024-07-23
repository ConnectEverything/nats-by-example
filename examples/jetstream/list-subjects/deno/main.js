import {
    connect,
    nuid
} from "https://deno.land/x/nats@v1.28.0/src/mod.ts";

const servers = Deno.env.get("NATS_URL")?.split(",");

const nc = await connect({ servers });

const jsm = await nc.jetstreamManager();
const js = nc.jetstream();

const randomName: () => string = function (): string {
    return nuid.next().substring(18)
};

// Create a stream with a few random subjects
const plainSubj = `PLAIN_${randomName()}`;
const gtSubj = `GT_${randomName()}`;
const starSubj = `STAR_${randomName()}`;
const name = `EVENTS_${randomName()}`;

let si = await jsm.streams.add({
    name,
    subjects: [plainSubj, `${gtSubj}.>`, `${starSubj}.*`]
});

console.log(`Stream: ${si.config.name} has these subjects: '${si.config.subjects}'`)

// ### Get stream info with StreamInfoRequestOptions
// Get the subjects via the streams.info call.
// Since this is "state" there are no subjects in the state unless
// there are messages in the subject.
si = await jsm.streams.info(name, {subjects_filter: ">"});

const count = si.state.subjects
    ? Object.getOwnPropertyNames(si.state.subjects).length
    : 0;
console.log(`Before publishing any messages, there should be 0 subjects in the state: ${count}`)

// console.log(`Before publishing any messages, there are 0 subjects: ${si.state.num_subjects}`)

// Publish a message
await js.publish(plainSubj)

si = await jsm.streams.info(name, {subjects_filter: ">"});
console.log("After publishing a message to a subject, it appears in state:")

if (si.state.subjects) {
    for (const [key, value] of Object.entries(si.state.subjects)) {
        console.log(`  subject '${key}' has ${value} message(s)`)
    }
}

// Publish some more messages, this time against wildcard subjects
await js.publish(`${gtSubj}.A`, "gtA-1");
await js.publish(`${gtSubj}.A`, "gtA-2");
await js.publish(`${gtSubj}.A.B`, "gtAB-1");
await js.publish(`${gtSubj}.A.B`, "gtAB-2");
await js.publish(`${gtSubj}.A.B.C`, "gtABC");
await js.publish(`${gtSubj}.B.B.B`, "gtBBB");
await js.publish(`${starSubj}.1`, "star1-1");
await js.publish(`${starSubj}.1`, "star1-2");
await js.publish(`${starSubj}.2`, "star2");

// Get all subjects
si = await jsm.streams.info(name, {subjects_filter: ">"});
console.log("Wildcard subjects show the actual subject, not the template.");

if (si.state.subjects) {
    for (const [key, value] of Object.entries(si.state.subjects)) {
        console.log(`  subject '${key}' has ${value} message(s)`)
    }
}

// ### Subject Filtering
// Instead of allSubjects, you can filter for a specific subject
si = await jsm.streams.info(name, {subjects_filter: `${gtSubj}.>`});
console.log(`Filtering the subject returns only matching entries ['${gtSubj}.>']`);

if (si.state.subjects) {
    for (const [key, value] of Object.entries(si.state.subjects)) {
        console.log(`  subject '${key}' has ${value} message(s)`)
    }
}

si = await jsm.streams.info(name, {subjects_filter: `${gtSubj}.A.>`});
console.log(`Filtering the subject returns only matching entries ['${gtSubj}.A>']`);

if (si.state.subjects) {
    for (const [key, value] of Object.entries(si.state.subjects)) {
        console.log(`  subject '${key}' has ${value} message(s)`)
    }
}

await nc.close();
