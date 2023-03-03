import os
import asyncio
import time

import nats
from nats.errors import TimeoutError, NoRespondersError


servers = os.environ.get("NATS_URL", "nats://localhost:4222").split(",")


async def main():
    
    nc = await nats.connect(servers=servers)
    print(nats.js.api.StreamConfig())

    js = nc.jetstream()
 
    await js.add_stream(name='events', subjects=['events.*']) 
    
    await js.publish('events.page_loaded',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.page_loaded',b'')
    await js.publish('events.mouse_clicked',b'')
    await js.publish('events.input_focused',b'')
    print("published 6 messages","\n")


    print(await js.streams_info(),"\n")

    await js.update_stream(name='events', subjects=['events.*'],max_msgs=10)
    print("set max messages to 10","\n")
    

    print(await js.streams_info(),"\n")
    
    await js.update_stream(name='events', subjects=['events.*'], max_msgs=10, max_bytes=300)
    print("set max bytes to 300","\n")
    

    print(await js.streams_info(),"\n")
    
    await js.update_stream(name='events', subjects=['events.*'], max_msgs=10, max_bytes=300, max_age=10)
    print("set max age to one second","\n")
    

    print(await js.streams_info(),"\n")
   
    time.sleep(10)

    print("10s")
    print(await js.streams_info())
   
    

if __name__ == '__main__':
    asyncio.run(main())