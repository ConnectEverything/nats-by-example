package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/nats.go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		urls            string
		natsContext     string
		preferredRegion string
		maxReconnects   int
		reconnectWait   time.Duration
		streamPrefix    string
	)

	flag.StringVar(&natsContext, "context", "", "NATS context.")
	flag.StringVar(&urls, "urls", "", "Regions URLs in the form of region=url,region=url.")
	flag.StringVar(&preferredRegion, "region", "", "Preferred region.")
	flag.IntVar(&maxReconnects, "max-reconnects", 5, "Maximum number of reconnects.")
	flag.DurationVar(&reconnectWait, "reconnect-wait", 2*time.Second, "Reconnect wait time.")
	flag.StringVar(&streamPrefix, "stream", "events", "Stream name.")

	flag.Parse()

	// Parse the URLs associated with each region.
	regionsURLs := make(map[string][]string)
	var regionOrder []string
	for _, u := range strings.Split(urls, ",") {
		parts := strings.Split(u, "=")
		if len(parts) != 2 {
			return fmt.Errorf("invalid url: %s", u)
		}
		if _, ok := regionsURLs[parts[0]]; !ok {
			regionOrder = append(regionOrder, parts[0])
		}
		regionsURLs[parts[0]] = append(regionsURLs[parts[0]], parts[1])
	}

	// This is not robust, since it relies on in-memory state. One could
	// use a database or a file to store the state, but this doesn't
	// work across instances.
	// A KV would work great, but falls into the same problem that the
	// stream does in terms of lagging behind across regions.
	conSeqs := NewConsumerSeqs()

	// This simulates publishing a publisher to the stream.
	startPublishers := func(nc *nats.Conn, region string) error {
		l := log.New(os.Stdout, fmt.Sprintf("[%s] ", region), log.LstdFlags)

		js, err := nc.JetStream()
		if err != nil {
			return err
		}

		subjectPrefix := fmt.Sprintf("%s.%s", streamPrefix, region)

		l.Printf("Starting publisher to %s.>", subjectPrefix)

		// Publish messages to the stream.
		var (
			i       int
			lastSeq uint64
		)

		for {
			i++
			ack, err := js.Publish(fmt.Sprintf("%s.%d", subjectPrefix, i), []byte(fmt.Sprintf("message %d", i)))
			if err == nil {
				lastSeq = ack.Sequence
			} else {
				l.Printf("error publishing: %s, last seq was %d", err, lastSeq)
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Setup the consumers on the stream, relative to the region.
	// This does two things, delete existing consumers and create new ones
	// with the correct start sequence.
	setupConsumers := func(nc *nats.Conn, region string) error {
		l := log.New(os.Stdout, fmt.Sprintf("[%s] ", region), log.LstdFlags)

		js, err := nc.JetStream()
		if err != nil {
			return err
		}

		streamName := fmt.Sprintf("%s-%s", streamPrefix, region)
		l.Printf("Setting up consumers for %s", streamName)

		// Given the region, delete existing consumers. Lookup the floor sequence
		// for the consumer and re-create.
		err = js.DeleteConsumer(streamName, "push-handler")
		if err != nil && !errors.Is(err, nats.ErrConsumerNotFound) {
			return err
		}

		err = js.DeleteConsumer(streamName, "pull-handler")
		if err != nil && !errors.Is(err, nats.ErrConsumerNotFound) {
			return err
		}

		pushStartSeq := conSeqs.Get(streamPrefix, "push-handler")
		if pushStartSeq > 0 {
			l.Printf("push: resuming from sequence %d", pushStartSeq+1)
		}

		pullStartSeq := conSeqs.Get(streamPrefix, "pull-handler")
		if pullStartSeq > 0 {
			l.Printf("pull: resuming from sequence %d", pullStartSeq+1)
		}

		// Default consumer configuration.
		pushCfg := nats.ConsumerConfig{
			Name:           "push-handler",
			AckPolicy:      nats.AckExplicitPolicy,
			DeliverSubject: "push-handler",
			DeliverPolicy:  nats.DeliverAllPolicy,
		}

		// Override if we have a start sequence.
		if pushStartSeq > 0 {
			pushCfg.DeliverPolicy = nats.DeliverByStartSequencePolicy
			pushCfg.OptStartSeq = pushStartSeq + 1
		}

		pullCfg := nats.ConsumerConfig{
			Name:          "pull-handler",
			AckPolicy:     nats.AckExplicitPolicy,
			DeliverPolicy: nats.DeliverAllPolicy,
		}

		if pullStartSeq > 0 {
			pullCfg.DeliverPolicy = nats.DeliverByStartSequencePolicy
			pullCfg.OptStartSeq = pullStartSeq + 1
		}

		_, err = js.AddConsumer(streamName, &pushCfg)
		if err != nil {
			return fmt.Errorf("push: %s", err)
		}

		_, err = js.AddConsumer(streamName, &pullCfg)
		if err != nil {
			return fmt.Errorf("pull: %s", err)
		}

		return nil
	}

	// This binds the subscriptions to the consumers.
	bindSubs := func(ctx context.Context, nc *nats.Conn, region string) error {
		l := log.New(os.Stdout, fmt.Sprintf("[%s] ", region), log.LstdFlags)

		js, err := nc.JetStream()
		if err != nil {
			return err
		}

		streamName := fmt.Sprintf("%s-%s", streamPrefix, region)

		var lastPushSeq uint64
		defer func() {
			l.Printf("push: last sequence was %d", lastPushSeq)
		}()

		_, err = js.Subscribe("", func(msg *nats.Msg) {
			md, _ := msg.Metadata()
			seq := md.Sequence.Stream

			err := msg.Ack()
			if err == nil {
				lastPushSeq = seq
				conSeqs.Set(streamPrefix, "push-handler", seq)
			} else {
				l.Printf("push: failed to ack message %d: %s", seq, err)
			}
		}, nats.Bind(streamName, "push-handler"))
		if err != nil {
			return fmt.Errorf("push: %w", err)
		}

		pullSub, err := js.PullSubscribe("", "", nats.Bind(streamName, "pull-handler"))
		if err != nil {
			return fmt.Errorf("pull: %w", err)
		}

		var lastPullSeq uint64
		defer func() {
			l.Printf("pull: last sequence was %d", lastPullSeq)
		}()
		for {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			msgs, err := pullSub.Fetch(10, nats.Context(cctx))
			cancel()
			if err != nil {
				// Expected if there are no messages.
				if err == context.DeadlineExceeded {
					continue
				}

				// If the context was canceled, we are done.
				if err == context.Canceled {
					return nil
				}

				// Otherwise, we have an error.
				return fmt.Errorf("pull: %w", err)
			}

			for _, msg := range msgs {
				md, _ := msg.Metadata()
				seq := md.Sequence.Stream

				err := msg.Ack()
				if err == nil {
					lastPullSeq = seq
					conSeqs.Set(streamPrefix, "pull-handler", seq)
				} else {
					l.Printf("pull: failed to ack message %d: %s", seq, err)
				}
			}
		}
	}

	connectHandler := func(ctx context.Context, nc *nats.Conn, region string) error {
		log.Printf("Connected to %q", region)

		if err := setupConsumers(nc, region); err != nil {
			return err
		}

		errch := make(chan error, 1)

		go func() {
			err := startPublishers(nc, region)
			if err != nil {
				errch <- err
			}
		}()

		go func() {
			err := bindSubs(ctx, nc, region)
			if err != nil {
				errch <- err
			}
		}()

		return <-errch
	}

	failoverHandler := func(current, next string) error {
		log.Printf("Failing over from %q to %q", current, next)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cm := &ConnManager{
		Context:         natsContext,
		RegionURLs:      regionsURLs,
		RegionOrder:     regionOrder,
		ReconnectWait:   reconnectWait,
		MaxReconnects:   maxReconnects,
		ConnectHandler:  connectHandler,
		FailoverHandler: failoverHandler,
	}

	err := cm.Start(ctx, preferredRegion)
	if err != nil {
		return err
	}
	defer cm.Drain()

	// Wait for an interrupt.
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	return nil
}

func NewConsumerSeqs() *ConsumerSeqs {
	return &ConsumerSeqs{
		seqs: make(map[[2]string]uint64),
	}
}

type ConsumerSeqs struct {
	seqs map[[2]string]uint64
	mux  sync.Mutex
}

// Get will fetch the sequence for the consumer.
func (cs *ConsumerSeqs) Get(stream, name string) uint64 {
	cs.mux.Lock()
	defer cs.mux.Unlock()
	k := [2]string{stream, name}
	return cs.seqs[k]
}

// Set will set the sequence for the consumer.
func (cs *ConsumerSeqs) Set(stream, name string, seq uint64) {
	cs.mux.Lock()
	defer cs.mux.Unlock()
	k := [2]string{stream, name}
	cs.seqs[k] = seq
}

type ConnectHandler func(ctx context.Context, nc *nats.Conn, region string) error
type FailoverHandler func(currentRegion string, nextRegion string) error

type ConnManager struct {
	Context         string
	RegionURLs      map[string][]string
	RegionOrder     []string
	ReconnectWait   time.Duration
	MaxReconnects   int
	ConnectHandler  ConnectHandler
	FailoverHandler FailoverHandler

	nc  *nats.Conn
	mux sync.Mutex

	currentRegion string
}

func (cm *ConnManager) Drain() {
	cm.mux.Lock()
	defer cm.mux.Unlock()

	if cm.nc != nil {
		cm.nc.Drain()
	}
}

func (cm *ConnManager) Start(ctx context.Context, region string) error {
	// Ensure the region is valid on start.
	_, ok := cm.RegionURLs[region]
	if !ok {
		return fmt.Errorf("invalid region: %s", region)
	}

	if cm.nc != nil && cm.nc.IsConnected() {
		return fmt.Errorf("already started")
	}

	return cm.setupConn(ctx, region)
}

func (cm *ConnManager) setupConn(ctx context.Context, region string) error {
	l := log.New(os.Stdout, fmt.Sprintf("[%s] ", region), log.LstdFlags)

	cm.mux.Lock()
	defer cm.mux.Unlock()

	// Ensure the region is valid.
	urls, ok := cm.RegionURLs[region]
	if !ok {
		return fmt.Errorf("invalid region: %s", region)
	}

	cm.currentRegion = region

	// The context is used for shared information, but the URL is overridden for the region.
	url := strings.Join(urls, ",")
	nctx, err := natscontext.New(cm.Context, true, natscontext.WithServerURL(url))
	if err != nil {
		return err
	}

	// Setup the connection for the specific region.
	var nc *nats.Conn
	nc, err = nctx.Connect(
		nats.ReconnectWait(cm.ReconnectWait),
		nats.MaxReconnects(cm.MaxReconnects),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			num, _, _ := sub.Pending()
			l.Printf("Subscription error: %s with %d pending", err, num)
		}),
		nats.ConnectHandler(func(nc *nats.Conn) {
			l.Printf("Connected to %q", nc.ConnectedUrlRedacted())
		}),
		nats.DisconnectHandler(func(nc *nats.Conn) {
			attempts := nc.Statistics.Reconnects
			l.Printf("Disconencted after %d attempts to %q", attempts, nc.ConnectedUrlRedacted())
			nc.Drain()

			// TODO: improve
			for _, r := range cm.RegionOrder {
				if r == cm.currentRegion {
					continue
				}
				err := cm.setupConn(ctx, r)
				if err == nil {
					break
				}
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			attempts := nc.Statistics.Reconnects
			l.Printf("Reconnected after %d attempts to %q", attempts, nc.ConnectedUrlRedacted())
		}),
	)
	if err != nil {
		return err
	}
	cm.nc = nc

	go func() {
		err := cm.ConnectHandler(ctx, nc, region)
		if err != nil {
			l.Printf("connect handler returned: %s", err)
		}
	}()

	return nil
}
