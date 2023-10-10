package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
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
		natsUrl         string
		natsUser        string
		natsPass        string
		natsCreds       string
		issuerSeed      string
		xkeySeed        string
		signingKeyFiles string
		usersFile       string
	)

	flag.StringVar(&natsUrl, "nats.url", nats.DefaultURL, "NATS server URL")
	flag.StringVar(&natsUser, "nats.user", "", "NATS user")
	flag.StringVar(&natsPass, "nats.pass", "", "NATS password")
	flag.StringVar(&natsCreds, "nats.creds", "", "NATS creds file")
	flag.StringVar(&issuerSeed, "issuer.seed", "", "Issuer seed")
	flag.StringVar(&xkeySeed, "xkey.seed", "", "Xkey seed")
	flag.StringVar(&signingKeyFiles, "signing.keys", "", "Signing keys")
	flag.StringVar(&usersFile, "users", "", "Users file")

	flag.Parse()

	// Parse the issuer account signing key.
	issuerKeyPair, err := fromSeedOrFile(issuerSeed)
	if err != nil {
		return fmt.Errorf("error parsing issuer seed: %s", err)
	}

	// Parse the xkey seed if present.
	var curveKeyPair nkeys.KeyPair
	if len(xkeySeed) > 0 {
		curveKeyPair, err = fromSeedOrFile(xkeySeed)
		if err != nil {
			return fmt.Errorf("error parsing xkey seed: %s", err)
		}
	}

	// Parse each of the account signing key files can create a map
	// of the local name to the key pair.
	var signingKeys map[string]*signingKey
	if len(signingKeyFiles) > 0 {
		signingKeys, err = loadSigningKeys(strings.Split(signingKeyFiles, ","))
		if err != nil {
			return fmt.Errorf("error loading signing keys: %s", err)
		}
	}

	// Load users from the users file, emulating an IAM backend.
	users, err := loadUsers(usersFile)
	if err != nil {
		return fmt.Errorf("error loading users: %s", err)
	}

	var opts []nats.Option
	if natsCreds != "" {
		opts = append(opts, nats.UserCredentials(natsCreds))
	} else {
		opts = append(opts, nats.UserInfo(natsUser, natsPass))
	}

	// Open the NATS connection passing the auth account creds file.
	nc, err := nats.Connect(natsUrl, opts...)
	if err != nil {
		return err
	}
	defer nc.Drain()

	// Helper function to construct an authorization response.
	respondMsg := func(msg *nats.Msg, userNkey, serverId, userJwt, errMsg string) {
		rc := jwt.NewAuthorizationResponseClaims(userNkey)
		rc.Audience = serverId
		rc.Error = errMsg
		rc.Jwt = userJwt

		// Sign the response with the issuer account.
		token, err := rc.Encode(issuerKeyPair)
		if err != nil {
			log.Printf("error encoding response JWT: %s", err)
			msg.Respond(nil)
			return
		}

		data := []byte(token)

		// Check if encryption is required.
		xkey := msg.Header.Get("Nats-Server-Xkey")
		if len(xkey) > 0 {
			data, err = curveKeyPair.Seal(data, xkey)
			if err != nil {
				log.Printf("error encrypting response JWT: %s", err)
				msg.Respond(nil)
				return
			}
		}

		log.Print("responding to authorization request")

		msg.Respond(data)
	}

	// Define the message handler for the authorization request.
	msgHandler := func(msg *nats.Msg) {
		var token []byte

		// Check for Xkey header and decrypt
		xkey := msg.Header.Get("Nats-Server-Xkey")
		if len(xkey) > 0 {
			if curveKeyPair == nil {
				respondMsg(msg, "", "", "", "xkey not supported")
				return
			}

			// Decrypt the message.
			token, err = curveKeyPair.Open(msg.Data, xkey)
			if err != nil {
				respondMsg(msg, "", "", "", fmt.Sprintf("error decrypting message: %s", err))
				return
			}
		} else {
			token = msg.Data
		}

		// Decode the authorization request claims.
		rc, err := jwt.DecodeAuthorizationRequestClaims(string(token))
		if err != nil {
			respondMsg(msg, "", "", "", err.Error())
			return
		}

		// Used for creating the auth response.
		userNkey := rc.UserNkey
		serverId := rc.Server.ID

		// Check if the user exists.
		userProfile, ok := users[rc.ConnectOptions.Username]
		if !ok {
			respondMsg(msg, userNkey, serverId, "", "user not found")
			return
		}

		// Check if the credential is valid.
		if userProfile.Pass != rc.ConnectOptions.Password {
			respondMsg(msg, userNkey, serverId, "", "invalid credentials")
			return
		}

		// Prepare a user JWT.
		uc := jwt.NewUserClaims(rc.UserNkey)
		uc.Name = rc.ConnectOptions.Username

		// Check if signing key is associated, otherwise assume non-operator mode
		// and set the audience to the account.
		var sk nkeys.KeyPair
		signingKey, ok := signingKeys[userProfile.Account]
		if !ok {
			uc.Audience = userProfile.Account
		} else {
			sk = signingKey.KeyPair
			uc.IssuerAccount = signingKey.PublicKey
		}

		// Set the associated permissions if present.
		uc.Permissions = userProfile.Permissions

		// Validate the claims.
		vr := jwt.CreateValidationResults()
		uc.Validate(vr)
		if len(vr.Errors()) > 0 {
			respondMsg(msg, userNkey, serverId, "", fmt.Sprintf("error validating claims: %s", vr.Errors()))
			return
		}

		// Sign it with the issuer key.
		ejwt, err := uc.Encode(sk)
		if err != nil {
			respondMsg(msg, userNkey, serverId, "", fmt.Sprintf("error signing user JWT: %s", err))
			return
		}

		respondMsg(msg, userNkey, serverId, ejwt, "")
	}

	// Create a queue subscription to the auth callout subject. This
	// allows for running multiple instances to distribute the load
	// and provide high availability.
	_, err = nc.QueueSubscribe("$SYS.REQ.USER.AUTH", "auth-callout", msgHandler)
	if err != nil {
		return err
	}

	// Block and wait for interrupt.
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	return nil
}

type signingKey struct {
	PublicKey string
	KeyPair   nkeys.KeyPair
}

func loadSigningKeys(keyFiles []string) (map[string]*signingKey, error) {
	keys := make(map[string]*signingKey)

	for _, item := range keyFiles {
		toks := strings.Split(item, ":")
		if len(toks) != 2 {
			return nil, fmt.Errorf("invalid signing key format: %s", item)
		}
		pk, fname := toks[0], toks[1]

		kp, err := fromSeedOrFile(fname)
		if err != nil {
			return nil, fmt.Errorf("error loading signing key: %s", err)
		}
		name := strings.SplitN(filepath.Base(fname), ".", 2)[0]
		keys[name] = &signingKey{PublicKey: pk, KeyPair: kp}
	}

	return keys, nil
}

// fromSeedOrFile will attempt to load a keypair from a seed file or
// or a literal seed string.
func fromSeedOrFile(seedOrFile string) (nkeys.KeyPair, error) {
	contents, err := ioutil.ReadFile(seedOrFile)
	if err != nil {
		if os.IsNotExist(err) {
			contents = []byte(seedOrFile)
		} else {
			return nil, fmt.Errorf("failed to read file: %s", err)
		}
	}

	return nkeys.FromSeed(contents)
}

// Model the user encoded in the users file.
type User struct {
	Pass        string
	Account     string
	Permissions jwt.Permissions
}

// Load and decode the users file.
func loadUsers(usersFile string) (map[string]*User, error) {
	users := make(map[string]*User)
	data, err := os.ReadFile(usersFile)
	if err != nil {
		return nil, fmt.Errorf("error reading user directory: %s", err)
	}

	err = json.Unmarshal(data, &users)
	if err != nil {
		return nil, fmt.Errorf("error parsing user directory: %s", err)
	}

	return users, nil
}
