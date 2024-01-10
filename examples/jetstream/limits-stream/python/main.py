import os
import asyncio
import time

import nats


servers = os.environ.get("NATS_URL", "nats://localhost:4222").split(",")


async def main():
    
    # Connect to NATS server
    nc = await nats.connect(servers=servers)
    
    # Get a context to produce and consume messages from NATS JetStream
    js = nc.jetstream()
    
    # Create a stream named 'events' and with subjects matching 'events.*'
    # 'events' will be a default stream that all events will be sent to
    # Storage parameter can be set to 'NONE' for no storage, 'FILE' for file based storage, or 'MEMORY' for memory based storage
    await js.add_stream(name='events', subjects=['events.*'], storage='file')
    
    # Publish 6 messages to the JetStream
    await js.publish('events.page_loaded',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.page_loaded',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.input_focused',b'')
    print("published 6 messages","\n")

    # Check the number of messages in the stream using streams_info
    # StreamState includes the total number of messages in the stream
    print(await js.streams_info(),"\n")

    # Update the 'events' stream to have a maximum of 10 messages
    await js.update_stream(name='events', subjects=['events.*'], max_msgs=10)
    print("set max messages to 10","\n")
    
    # Check the number of messages in the stream using streams_info
    # StreamState includes the total number of messages in the stream
    print(await js.streams_info(),"\n")
    
    # Update the 'events' stream to have a maximum of 300 bytes
    await js.update_stream(name='events', subjects=['events.*'], max_msgs=10, max_bytes=300)
    print("set max bytes to 300","\n")
    
    # Check the number of messages in the stream using streams_info
    # StreamState includes the total number of messages in the stream
    print(await js.streams_info(),"\n")
    
    # Update the 'events' stream to have a maximum age of 0.1 seconds
    await js.update_stream(name='events', subjects=['events.*'], max_msgs=10, max_bytes=300, max_age=0.1)
    print("set max age to 0.1 second","\n")
    
    # Check the number of messages in the stream using streams_info
    # StreamState includes the total number of messages in the stream
    print(await js.streams_info(),"\n")
    
    # Sleep for 10 seconds to allow messages to expire
    time.sleep(10)
    
    # Check the number of messages in the stream using streams_info
    # StreamState includes the total number of messages in the stream
    print(await js.streams_info())
    
    # Delete the 'events' stream
    await js.delete_stream('events')


if __name__ == '__main__':
    asyncio.run(main())
