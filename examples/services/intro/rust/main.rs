// In this sample, we'll use the Rust SDK to create and start a service, which automatically
// participates in the service discovery, stats management, and ping operations. The service
// we're going to build exposes two endpoints:

// * min - returns the minimum value in an array of integers
// * max - returns the maximum value in an array of integers

use async_nats::service::ServiceExt;
use futures::StreamExt;

#[tokio::main]
async fn main() -> Result<(), async_nats::Error> {
    // Use the NATS_URL env variable if defined, otherwise fallback
    // to the default.
    let nats_url =
        std::env::var("NATS_URL").unwrap_or_else(|_| "nats://localhost:4222".to_string());

    // This establishes a connection to the NATS server
    let client = async_nats::connect(nats_url).await?;

    // Uses the `service_builder` extension function to add a service definition to
    // the NATS client. As soon as `start` is called, the service is now visible
    // and available for interrogation and discovery.
    let service = client
        .service_builder()
        .description("A handy min max service")
        .start("minmax", "0.0.1")
        .await?;

    // Everything within this group will default to responding on a subject with a prefix
    // of `minmax.`
    let g = service.group("minmax");

    // Adds the `min` endpoint to the `minmax` group. If we don't specify an override subject,
    // then this endpoint will be listening on `minmax.min`
    let mut min = g.endpoint("min").await?;

    // Adds the `max` endpoint to the `minmax` group which will be listening on the
    // `minmax.max` subject.
    let mut max = g.endpoint("max").await?;

    // Spawns a background loop that iterates over the stream of incoming requests. Note
    // that in order for service stats to update properly, you have to use the `respond`
    // function rather than manually publishing on a reply-to subject.
    tokio::spawn(async move {
        while let Some(request) = min.next().await {
            // The input to this endpoint is a JSON array of integers and the function
            // returns a string with the min value
            let ints = decode_input(&request.message.payload);
            let res = format!("{}", ints.iter().min().unwrap());
            request.respond(Ok(res.into())).await.unwrap();
        }
    });

    // Spawns a background loop to iterate over the stream of requests for the `max`
    // endpoint. Returns a string containing the maximum value of the input array
    tokio::spawn(async move {
        while let Some(request) = max.next().await {
            let ints = decode_input(&request.message.payload);
            let res = format!("{}", ints.iter().max().unwrap());
            request.respond(Ok(res.into())).await.unwrap();
        }
    });

    // Now let's exercise the service we've deployed and advertised


    // All of our calls will use the same input bytes so we can create this just once
    let input_vec = vec![-1, 2, 100, -2000];
    let input_bytes = serde_json::to_vec(&input_vec).unwrap();

    // Make a request on `minmax.min` and obtain the result
    let min_res = client
        .request("minmax.min", input_bytes.clone().into())
        .await?;

    // Make a request on `minmax.max` and obtain the result
    let max_res = client
        .request("minmax.max", input_bytes.into())
        .await?;

    // Finally output our results
    println!(
        "minimum: {}\nmaximum: {}",
        std::str::from_utf8(&min_res.payload).unwrap(),
        std::str::from_utf8(&max_res.payload).unwrap(),
    );

    Ok(())
}

// This is just a simple utility function to decode the input from a JSON array
fn decode_input(raw: &[u8]) -> Vec<i32> {
    serde_json::from_slice(raw).unwrap()
}
