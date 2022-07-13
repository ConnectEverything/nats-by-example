use futures::StreamExt;
use std::{env, str::from_utf8};

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL").unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // Publish a message to the subject `greet.joe`.
    client
        .publish("greet.joe".to_string(), "hello".into())
        .await?;

    // `Subscriber` implements Rust iterator, so we can leverage
    // combinators like `take()` to limit the messages intended
    // to be consumed for this interaction.
    let mut subscription = client
        .subscribe("greet.*".to_string())
        .await?
        .take(3);

    // Publish to three different subjects matching the wildcard.
    client
        .publish("greet.sue".to_string(), "hello".into())
        .await?;
    client
        .publish("greet.bob".to_string(), "hello".into())
        .await?;
    client
        .publish("greet.pam".to_string(), "hello".into())
        .await?;

    // Notice that the first message received is `greet.sue` and not
    // `greet.joe` which was the first message published. This is because
    // core NATS provides at-most-once quality of service (QoS). Subscribers
    // must be connected showing *interest* in a subject for the server to
    // relay the message to the client.
    while let Some(message) = subscription.next().await {
        println!("{:?} received on {:?}", from_utf8(&message.payload), &message.subject);
    }

    Ok(())
}

