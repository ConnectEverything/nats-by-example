use std::{env, str::from_utf8};

use futures::stream::StreamExt;

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL").unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // Core NATS `client.subscribe()` can subscribe to more than one explicit subject using
    // `>` and `*` wildcards, but not to two or more distinct subjects.
    // However, it's still possible to have just one stream of messages for such use case.
    //
    // First, let's subscribe to desired subjects.
    let subscription_cars = client.subscribe("cars.>".into()).await?;
    let subscription_planes = client.subscribe("planes.>".into()).await?;
    let subscription_ships = client.subscribe("ships.>".into()).await?;

    // Then, spawn three tasks that publishes to each subject at the same time.
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
                    .publish(format!("ships.{}", i), format!("ship number {}", i).into())
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

    // Now, let's create the stream that will receive messages from all subscriptions.
    // We also leverage the `take()` combinator to limit number of messages we want to get in total.
    //
    // If you need to get messages from at most two subscriptions, `futures::stream::select` can be
    // used instead.
    let mut messages =
        futures::stream::select_all([subscription_cars, subscription_planes, subscription_ships])
            .take(300);

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
