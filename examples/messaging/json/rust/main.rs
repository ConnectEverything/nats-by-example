use std::env;
use serde_json::json;
use serde::{Deserialize,Serialize};
use futures::stream::StreamExt;

// Use the macro to generate the serialize and deserialize methods on
// a struct with basic types.
#[derive(Serialize, Deserialize)]
struct Payload {
    foo: String,
    bar: u8,
}

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL")
        .unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // Create a subscription that receives two messages. One message will
    // contain a valid serialized payload and the other will not.
    let mut subscriber = client.subscribe("foo").await?.take(2);

    // Construct a Payload value and serialize it.
    let payload = Payload{foo: "bar".to_string(), bar: 27};
    let bytes = serde_json::to_vec(&json!(payload))?;

    // Publish the serialized payload.
    client.publish("foo", bytes.into()).await?;
    client.publish("foo", "not json".into()).await?;

    // Loop through the expected messages and  attempt to deserialize the payload
    // into a Payload value. If deserialization into this type fails, alternate
    // handling can be performed, either discarding or attempting to derialize in
    // a more general type (such as a map).
    while let Some(message) = subscriber.next().await {
        if let Ok(payload) = serde_json::from_slice::<Payload>(message.payload.as_ref()) {
            println!("received valid JSON payload: foo={:?} bar={:?}", payload.foo, payload.bar);
        } else {
            println!("received invalid JSON payload: {:?}", message.payload);
        }
    }

    Ok(())
}
