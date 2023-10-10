package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		natsUrl    string
		natsUser   string
		natsPass   string
		issuerSeed string
		xkeySeed   string
		usersFile  string
	)

	flag.StringVar(&natsUrl, "nats.url", nats.DefaultURL, "NATS server URL")
	flag.StringVar(&natsUser, "nats.user", "", "NATS user")
	flag.StringVar(&natsPass, "nats.pass", "", "NATS password")
	flag.StringVar(&issuerSeed, "issuer.seed", "", "Issuer seed")
	flag.StringVar(&xkeySeed, "xkey.seed", "", "Xkey seed")
	flag.StringVar(&usersFile, "users", "", "Users file")

	flag.Parse()

	// Parse the issuer account signing key.
	issuerKeyPair, err := nkeys.FromSeed([]byte(issuerSeed))
	if err != nil {
		return fmt.Errorf("error parsing issuer seed: %s", err)
	}

	// Parse the xkey seed if present.
	var curveKeyPair nkeys.KeyPair
	if len(xkeySeed) > 0 {
		curveKeyPair, err = nkeys.FromSeed([]byte(xkeySeed))
		if err != nil {
			return fmt.Errorf("error parsing xkey seed: %s", err)
		}
	}

	// Model the user encoded in the users file.
	type User struct {
		Pass        string
		Account     string
		Permissions jwt.Permissions
	}

	// Load and decode the users file.
	users := make(map[string]*User)
	data, err := os.ReadFile(usersFile)
	if err != nil {
		return fmt.Errorf("error reading user directory: %s", err)
	}
	err = json.Unmarshal(data, &users)
	if err != nil {
		return fmt.Errorf("error parsing user directory: %s", err)
	}

	// Open the NATS connection passing the auth account creds file.
	nc, err := nats.Connect(natsUrl, nats.UserInfo(natsUser, natsPass))
	if err != nil {
		return err
	}
	defer nc.Drain()

	// Helper function to construct an authorization response.
	respondMsg := func(req micro.Request, userNkey, serverId, userJwt, errMsg string) {
		rc := jwt.NewAuthorizationResponseClaims(userNkey)
		rc.Audience = serverId
		rc.Error = errMsg
		rc.Jwt = userJwt

		token, err := rc.Encode(issuerKeyPair)
		if err != nil {
			log.Printf("error encoding response JWT: %s", err)
			req.Respond(nil)
			return
		}

		data := []byte(token)

		// Check if encryption is required.
		xkey := req.Headers().Get("Nats-Server-Xkey")
		if len(xkey) > 0 {
			data, err = curveKeyPair.Seal(data, xkey)
			if err != nil {
				log.Printf("error encrypting response JWT: %s", err)
				req.Respond(nil)
				return
			}
		}

		req.Respond(data)
	}

	// Define the message handler for the authorization request.
	msgHandler := func(req micro.Request) {
		var token []byte

		// Check for Xkey header and decrypt
		xkey := req.Headers().Get("Nats-Server-Xkey")
		if len(xkey) > 0 {
			if curveKeyPair == nil {
				respondMsg(req, "", "", "", "xkey not supported")
				return
			}

			// Decrypt the message.
			token, err = curveKeyPair.Open(req.Data(), xkey)
			if err != nil {
				respondMsg(req, "", "", "", "error decrypting message")
				return
			}
		} else {
			token = req.Data()
		}

		// Decode the authorization request claims.
		rc, err := jwt.DecodeAuthorizationRequestClaims(string(token))
		if err != nil {
			respondMsg(req, "", "", "", err.Error())
			return
		}

		// Used for creating the auth response.
		userNkey := rc.UserNkey
		serverId := rc.Server.ID

		// Check if the user exists.
		userProfile, ok := users[rc.ConnectOptions.Username]
		if !ok {
			respondMsg(req, userNkey, serverId, "", "user not found")
			return
		}

		// Check if the credential is valid.
		if userProfile.Pass != rc.ConnectOptions.Password {
			respondMsg(req, userNkey, serverId, "", "invalid credentials")
			return
		}

		// Prepare a user JWT.
		uc := jwt.NewUserClaims(rc.UserNkey)
		uc.Name = rc.ConnectOptions.Username

		// Audience contains the account in non-operator mode.
		uc.Audience = userProfile.Account

		// Set the associated permissions if present.
		uc.Permissions = userProfile.Permissions

		// Validate the claims.
		vr := jwt.CreateValidationResults()
		uc.Validate(vr)
		if len(vr.Errors()) > 0 {
			respondMsg(req, userNkey, serverId, "", "error validating claims")
			return
		}

		// Sign it with the issuer key since this is non-operator mode.
		ejwt, err := uc.Encode(issuerKeyPair)
		if err != nil {
			respondMsg(req, userNkey, serverId, "", "error signing user JWT")
			return
		}

		respondMsg(req, userNkey, serverId, ejwt, "")
	}

	// Create a service for auth callout with an endpoint binding to
	// the required subject. This allows for running multiple instances
	// to distribute the load, observe stats, and provide high availability.
	srv, err := micro.AddService(nc, micro.Config{
		Name:        "auth-callout",
		Version:     "0.0.1",
		Description: "Auth callout service.",
	})
	if err != nil {
		return err
	}

	g := srv.
		AddGroup("$SYS").
		AddGroup("REQ").
		AddGroup("USER")

	err = g.AddEndpoint("AUTH", micro.HandlerFunc(msgHandler))
	if err != nil {
		return err
	}

	// Block and wait for interrupt.
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	return nil
}
