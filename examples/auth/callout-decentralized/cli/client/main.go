package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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
		natsURL   string
		credsFile string
		user      string
		pass      string
	)

	flag.StringVar(&natsURL, "url", nats.DefaultURL, "NATS URL")
	flag.StringVar(&credsFile, "creds", "", "NATS credentials file")
	flag.StringVar(&user, "user", "", "NATS user")
	flag.StringVar(&pass, "pass", "", "NATS password")

	flag.Parse()

	nc, err := nats.Connect(
		natsURL,
		nats.UserCredentials(credsFile),
		nats.UserInfo(user, pass),
	)
	if err != nil {
		return err
	}
	defer nc.Drain()

	log.Printf("%s connected to %s", user, nc.ConnectedUrl())

	return nil
}
