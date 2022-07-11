use std::env;
use futures::StreamExt;

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the environment variable provided by the container or fallback
    // to the default URL.
    let nats_url = env::var("NATS_URL").unwrap_or("nats://localhost:4222".to_string());

    // Create the connection.
    let nc = async_nats::connect(nats_url).await?;

    // Publish a message on a subject. This will be discarded since
    // there is no interest for the subject via a subscription.
    nc.publish("greet.joe".into(), "hello".into()).await?;

    // Create a subscription to show interest. This uses a single token
    // wildcard, `*`.
    let mut sub = nc.subscribe("greet.*".into()).await?;

    // Publish some messages with different tokens.
    nc.publish("greet.joe".into(), "hello".into()).await?;
    nc.publish("greet.pam".into(), "hello".into()).await?;
    nc.publish("greet.sue".into(), "hello".into()).await?;

    // Read the next three messages.
    let msg1 = sub.next().await?;
    println!("Received: {:?}", msg1.data);

    let msg2 = sub.next().await?;
    println!("Received: {:?}", msg2.data);

    let msg2 = sub.next().await?;
    println!("Received: {:?}", msg3.data);

    sub.unsubscribe().await?;
    nc.flush().await?;

    Ok(())
}
