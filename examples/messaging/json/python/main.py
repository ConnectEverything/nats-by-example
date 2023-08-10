import asyncio
import json
import os
from dataclasses import asdict, dataclass

import nats


@dataclass
class Payload:
    foo: str
    bar: int


async def main():
    # Use the NATS_URL env variable if defined, otherwise fallback to the default.
    nats_url = os.getenv("NATS_URL", "nats://localhost:4222")

    client = await nats.connect(nats_url)

    # Create a subscription that receives two messages. One message will
    # contain a valid serialized payload and the other will not.
    subscriber = await client.subscribe("foo", max_msgs=2)

    # Construct a Payload value and serialize it.
    payload = Payload(foo="bar", bar=27)
    bytes_ = json.dumps(asdict(payload)).encode()

    # Publish the serialized payload.
    await client.publish("foo", bytes_)
    await client.publish("foo", b"not json")

    # Loop through the expected messages and  attempt to deserialize the payload
    # into a Payload value. If deserialization into this type fails, alternate
    # handling can be performed, either discarding or attempting to derialize in
    # a more general type (such as an untyped dict).
    async for message in subscriber.messages:
        try:
            payload = Payload(**json.loads(message.data))
            print(f"received valid JSON payload: {payload.foo=} {payload.bar=}")
        except json.decoder.JSONDecodeError:
            print(f"received invalid JSON payload: {message.data=}")


if __name__ == "__main__":
    asyncio.run(main())
