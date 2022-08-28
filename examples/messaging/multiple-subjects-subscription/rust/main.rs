use std::{env, str::from_utf8};

use futures::stream::StreamExt;

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL").unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // Core NATS `client.subscribe()` can subscribe to more than one explicit subject using
    // `>` and `*` wildcards, but not to two distinct subjects.
    // However, it's still possible to have just one stream of messages for such case.
    //
    // First, let's subscribe to those subjects.
    let subscription_cars = client.subscribe("cars.>".into()).await?;
    let subscription_planes = client.subscribe("planes.>".into()).await?;

    // Then, spawn two tasks that publishes to both subject at the same time.
    tokio::task::spawn({
        let client = client.clone();
        async move {
            for i in 0..100 {
                client
                    .publish(format!("cars.{}", i), format!("car number {}", i).into())
                    .await?;
            }
            Ok::<(), async_nats::Error>(())
        }
    });
    tokio::task::spawn({
        let client = client.clone();
        async move {
            for i in 0..100 {
                client
                    .publish(
                        format!("planes.{}", i),
                        format!("plane number {}", i).into(),
                    )
                    .await?;
            }
            Ok::<(), async_nats::Error>(())
        }
    });

    // Now, let's create the cars stream that will receive messages from both subscriptions.
    // If you need more than two subscriptions, consider using `futures::stream::select_all`.
    // We also leverage the `take()` combinator to limit number of messages we want to get in total.
    let mut messages = futures::stream::select(subscription_cars, subscription_planes).take(200);

    // Receive the messages from both subjects.
    while let Some(message) = messages.next().await {
        println!(
            "received message on subject {} with paylaod {}",
            message.subject,
            from_utf8(&message.payload)?
        );
    }

    Ok(())
}
