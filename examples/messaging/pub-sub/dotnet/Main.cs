using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

using NATS.Client;

// Create a new connection factory to create
// a connection.
Options opts = ConnectionFactory.GetDefaultOptions();
opts.Url = "nats://nats:4222";

// Creates a live connection to the default
// NATS Server running locally
ConnectionFactory cf = new ConnectionFactory();
IConnection c = cf.CreateConnection(opts);

// Setup an event handler to process incoming messages.
// An anonymous delegate function is used for brevity.
EventHandler<MsgHandlerEventArgs> h = (sender, args) =>
{
    // print the message
    Console.WriteLine(args.Message);

    // Here are some of the accessible properties from
    // the message:
    // args.Message.Data;
    // args.Message.Reply;
    // args.Message.Subject;
    // args.Message.ArrivalSubcription.Subject;
    // args.Message.ArrivalSubcription.QueuedMessageCount;
    // args.Message.ArrivalSubcription.Queue;

    // Unsubscribing from within the delegate function is supported.
    args.Message.ArrivalSubcription.Unsubscribe();
};

// The simple way to create an asynchronous subscriber
// is to simply pass the event in.  Messages will start
// arriving immediately.
IAsyncSubscription s = c.SubscribeAsync("foo", h);

// Alternatively, create an asynchronous subscriber on subject foo,
// assign a message handler, then start the subscriber.   When
// multicasting delegates, this allows all message handlers
// to be setup before messages start arriving.
IAsyncSubscription sAsync = c.SubscribeAsync("foo");
sAsync.MessageHandler += h;
sAsync.Start();

// Simple synchronous subscriber
ISyncSubscription sSync = c.SubscribeSync("foo");

// Using a synchronous subscriber, gets the first message available,
// waiting up to 1000 milliseconds (1 second)
Msg m = sSync.NextMessage(1000);

c.Publish("foo", Encoding.UTF8.GetBytes("hello world"));

// Unsubscribing
sAsync.Unsubscribe();

// Publish requests to the given reply subject:
c.Publish("foo", "bar", Encoding.UTF8.GetBytes("help!"));

// Sends a request (internally creates an inbox) and Auto-Unsubscribe the
// internal subscriber, which means that the subscriber is unsubscribed
// when receiving the first response from potentially many repliers.
// This call will wait for the reply for up to 1000 milliseconds (1 second).
m = c.Request("foo", Encoding.UTF8.GetBytes("help"), 1000);

// Draining and closing a connection
c.Drain();

// Closing a connection
c.Close();
