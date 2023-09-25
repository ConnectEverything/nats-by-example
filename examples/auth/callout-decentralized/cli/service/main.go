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
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

const (
	AuthCalloutSubject     = "$SYS.REQ.USER.AUTH"
	AuthInfoSubject        = "$SYS.REQ.USER.INFO"
	AuthRequestJWTAudience = "nats-authorization-request"
	AuthAccountSigHeader   = "Auth-Account-Sig"
	AuthAccountPkHeader    = "Auth-Account-Pk"
	AuthCalloutServerIdTag = "Auth-Callout-Server-ID"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		natsUrl            string
		natsCreds          string
		natsAccount        string
		natsTargetAccounts string
		userDir            string
	)

	flag.StringVar(&natsUrl, "nats", nats.DefaultURL, "NATS server URL")
	flag.StringVar(&natsCreds, "creds", "", "NATS auth credentials file")
	flag.StringVar(&natsAccount, "account", "", "NATS auth account nkey file")
	flag.StringVar(&natsTargetAccounts, "targets", "", "Comma-separated list of account nkey files")
	flag.StringVar(&userDir, "users", "", "User directory file")

	flag.Parse()

	log.SetPrefix("[auth] ")

	// Read the credentials file to extract the seed in order to sign
	// the authorization response JWTs.
	authKey, err := loadAuthKey(natsAccount)
	if err != nil {
		return fmt.Errorf("error reading creds file: %s", err)
	}

	log.Printf("loaded auth key: %s", authKey)

	// Parse each of the account signing key files can create a map
	// of the local name to the key pair.
	accountKeys, err := loadAccountKeys(strings.Split(natsTargetAccounts, ","))
	if err != nil {
		return fmt.Errorf("error loading account keys: %s", err)
	}

	for name := range accountKeys {
		log.Printf("loaded account key: %s", name)
	}

	// Emulate a user directory which assigns the account
	// and the permissions.
	userDirectory, err := loadUserDirectory(userDir)
	if err != nil {
		return fmt.Errorf("error loading user directory: %s", err)
	}

	for name := range userDirectory {
		log.Printf("loaded user: %s", name)
	}

	// Open the NATS connection passing the auth account creds file.
	nc, err := nats.Connect(natsUrl, nats.UserCredentials(natsCreds))
	if err != nil {
		return err
	}
	defer nc.Drain()

	msgHandler := func(msg *nats.Msg) {
		// Decode the authorization request claims.
		rc, err := jwt.DecodeAuthorizationRequestClaims(string(msg.Data))
		if err != nil {
			respondMsg(msg, "", err.Error())
			return
		}

		b, _ := json.MarshalIndent(rc, "", "  ")
		log.Printf("authorization request: %s", string(b))

		//
		// Authentication stage...
		//

		// Extract out the credentials.
		user := rc.ConnectOptions.Username
		pass := rc.ConnectOptions.Password

		// Check if the user exists.
		userProfile, ok := userDirectory[user]
		if !ok {
			log.Printf("user not found: %s", user)
			respondMsg(msg, "", "user not found")
			return
		}

		// Check if the credential is valid.
		if userProfile.Pass != pass {
			log.Printf("invalid credentials for user: %s", user)
			respondMsg(msg, "", "invalid credentials")
			return
		}

		// Check if a NATS account is associated with this user.
		if userProfile.NATS.Account == "" {
			log.Printf("user not associated with a NATS account: %s", user)
			respondMsg(msg, "", "user not associated with a NATS account")
			return
		}

		// Check the if signing key exists for this account.
		accountKP, ok := accountKeys[userProfile.NATS.Account]
		if !ok {
			log.Printf("NATS account not found: %s", userProfile.NATS.Account)
			respondMsg(msg, "", "NATS account not found")
			return
		}

		//
		// Authorization prep...
		//

		// Prepare a user JWT
		uc := jwt.NewUserClaims(rc.UserNkey)
		uc.Name = user

		// Set the associated permissions.
		uc.Permissions = userProfile.NATS.Permissions

		// Set an optional expiry that will result in the server forcing the
		// client to re-authenticate after that time.
		uc.Expires = time.Now().Add(10 * time.Minute).Unix()

		// Set the issuer as the target account (via the signing key).
		uc.Issuer, _ = accountKP.PublicKey()

		// Set a tag indicating the user the original request came from.
		uc.Tags.Add(fmt.Sprintf("%s:%s", AuthCalloutServerIdTag, rc.Server.ID))

		// Validate the claims.
		vr := jwt.CreateValidationResults()
		uc.Validate(vr)
		if len(vr.Errors()) > 0 {
			log.Printf("error validating claims: %s", vr.Errors())
			respondMsg(msg, "", fmt.Sprintf("error validating claims: %s", vr.Errors()))
			return
		}

		// Sign it with the target account signing key.
		ejwt, err := uc.Encode(accountKP)
		if err != nil {
			log.Printf("error encoding user JWT: %s", err)
			respondMsg(msg, "", "error encoding user JWT")
			return
		}

		// Prepare the authorization response claims.
		cr := jwt.NewAuthorizationResponseClaims(rc.UserNkey)
		cr.Audience = rc.Server.ID
		cr.Jwt = ejwt
		rjwt, err := cr.Encode(authKey)
		if err != nil {
			log.Printf("error encoding response JWT: %s", err)
			respondMsg(msg, "", "error encoding response JWT")
			return
		}

		respondMsg(msg, rjwt, "")
	}

	// Create a queue subscription to the auth callout subject. This
	// allows for running multiple instances to distribute the load
	// and provide high availability.
	_, err = nc.QueueSubscribe(AuthCalloutSubject, "auth-callout", msgHandler)
	if err != nil {
		return err
	}

	fmt.Println("auth service: listening for auth requests")

	// Block and wait for interrupt.
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	return nil
}

type CalloutResponse struct {
	Error     string `json:"error,omitempty"`
	UserToken string `json:"user_token,omitempty"`
}

func respondMsg(msg *nats.Msg, err string, userToken string) {
	cr := CalloutResponse{
		Error:     err,
		UserToken: userToken,
	}
	b, _ := json.Marshal(cr)
	rc := jwt.NewAuthorizationResponseClaims()
	vr := jwt.CreateValidationResults()
	rc.Validate(vr)
	if len(vr.Errors()) > 0 {
		log.Printf("error validating claims: %s", vr.Errors())
		respondMsg(msg, "", fmt.Sprintf("error validating claims: %s", vr.Errors()))
		return
	}

	msg.Respond(b)
}

func loadAuthKey(fname string) (nkeys.KeyPair, error) {
	contents, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("error reading seed file: %s", err)
	}
	kp, err := nkeys.FromSeed(contents)
	if err != nil {
		return nil, fmt.Errorf("error parsing seed: %s", err)
	}
	pk, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("error extracting public key: %s", err)
	}
	// Check this is an account key pair.
	if pk[0] != 'A' {
		return nil, fmt.Errorf("invalid account key: %s", pk)
	}
	return kp, nil
}

func loadAccountKeys(keyFiles []string) (map[string]nkeys.KeyPair, error) {
	accountKeys := make(map[string]nkeys.KeyPair)
	for _, fname := range keyFiles {
		contents, err := ioutil.ReadFile(fname)
		if err != nil {
			return nil, fmt.Errorf("error reading seed file: %s", err)
		}
		kp, err := nkeys.FromSeed(contents)
		if err != nil {
			return nil, fmt.Errorf("error parsing seed: %s", err)
		}
		pk, err := kp.PublicKey()
		if err != nil {
			return nil, fmt.Errorf("error extracting public key: %s", err)
		}
		// Check this is an account key pair.
		if pk[0] != 'A' {
			return nil, fmt.Errorf("invalid account key: %s", pk)
		}
		// Get the basename of the file for local name.
		name := filepath.Base(fname)
		accountKeys[name] = kp
	}

	return accountKeys, nil
}

type User struct {
	Pass string
	NATS struct {
		Account     string
		Permissions jwt.Permissions
	}
}

func loadUserDirectory(file string) (map[string]*User, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("error reading user directory: %s", err)
	}

	dir := make(map[string]*User)
	err = json.Unmarshal(data, &dir)
	if err != nil {
		return nil, fmt.Errorf("error parsing user directory: %s", err)
	}

	return dir, nil
}
