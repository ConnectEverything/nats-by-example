use futures::stream::StreamExt;
use rand::Rng;
use std::{env, str::from_utf8, time::Duration};

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL")
        .unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // `Subscriber` implements Rust iterator, so we can leverage
    // combinators like `take()` to limit the messages intended
    // to be consumed for this interaction.
    let subscription = client.subscribe("greet.*".to_string()).await?.take(50);

    // Publish set of messages, each with order identifier.
    for i in 0..50 {
        client
            .publish("greet.joe".to_string(), format!("hello {}", i).into())
            .await?;
    }

    // Iterate over messages concurrently.
    // `for_each_concurrent` allows us to not wait for time-consuming operation and receive next
    // message immediately.
    // `25` is a limit for concurrent operations.
    subscription
        .for_each_concurrent(25, |message| async move {
            // Let's simulate expensive operation.
            let num = rand::thread_rng().gen_range(0..500);
            tokio::time::sleep(Duration::from_millis(num)).await;
            // Print the result after sleep.
            println!(
                "received message: {:?}",
                from_utf8(&message.payload).unwrap()
            )
        })
        .await;

    Ok(())
}
