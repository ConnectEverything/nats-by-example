#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url = env::var("NATS_URL")
        .unwrap_or_else(|_| "nats://localhost:4222".to_string());

    let client = async_nats::connect(nats_url).await?;

    // In addition to vanilla publish-request, NATS supports request-reply
    // interactions as well. Under the covers, this is just an optimized
    // pair of publish-subscribe operations.
    // The _requests_ is just a subscription that _responds_ to a message
    // sent to it. This kind of subscription is called a _service_.
    let mut requests = client.subscribe("greet.*".into()).await.unwrap();

    // Spawn a new task, so we can response to incoming requests.
    // Usually request/response happens across clients and network
    // and in such scenarios, you don't need a separate task.
    tokio::spawn({
        let client = client.clone();
        async move {
            // Iterate over requests.
            while let Some(request) = requests.next().await {
                // Check if message we got have a `reply` to which we can publish the response.
                if let Some(reply) = request.reply {
                    // Publish the response.
                    let name = &request.subject[6..];
                    client
                        .publish(reply, format!("hello, {}", name).into())
                        .await?;
                }
            }
            Ok::<(), async_nats::Error>(())
        }
    });

    // As there is a `Subscriber` listening to requests, we can sent those.
    // We're leaving the payload empty for these examples.
    let response = client.request("greet.sue".into(), "".into()).await?;
    println!("got a response: {:?}", from_utf8(&response.payload)?);

    let response = client.request("greet.john".into(), "".into()).await?;
    println!("got a response: {:?}", from_utf8(&response.payload)?);

    // If we don't want to endlessly wait until response is returned, we can wrap
    // it in `tokio::time::timeout`.
    let response = tokio::time::timeout(
        Duration::from_millis(500),
        client.request("greet.bob".into(), "".into()),
    )
    // first `?` is `Err` if timeout occurs, second is for actual response `Result`.
    .await??;
    println!("got a response: {:?}", from_utf8(&response.payload)?);

    Ok(())
}
