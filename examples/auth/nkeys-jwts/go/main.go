package main

import (
	"fmt"
	"log"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func main() {
	log.SetFlags(0)

	// Create a one-off operator keypair for the purpose of this example.
	// In practice, the operator needs to be created ahead of time to configure
	// the resolver in the server config if you are deploying your own NATS server.
	// This most commonly done using the "nsc" tool:
	// ```
	// nsc add operator --generate-signing-key --sys --name local
	// nsc edit operator --require-signing-keys --account-jwt-server-url nats://127.0.0.1:4222
	// ```
	// Signing keys are technically optional, but a best practice.
	operatorKP, _ := nkeys.CreateOperator()

	// We can distinguish operators, accounts, and users by the first character
	// of their public key: O, A, or U.
	operatorPub, _ := operatorKP.PublicKey()
	fmt.Printf("operator pubkey: %s\n", operatorPub)

	// Seed values (private key), are prefixed with S.
	operatorSeed, _ := operatorKP.Seed()
	fmt.Printf("operator seed: %s\n\n", string(operatorSeed))

	// To create accounts on demand, we start with creatinng a new keypair which
	// has a unique ID.
	accountKP, _ := nkeys.CreateAccount()

	accountPub, _ := accountKP.PublicKey()
	fmt.Printf("account pubkey: %s\n", accountPub)

	accountSeed, _ := accountKP.Seed()
	fmt.Printf("account seed: %s\n", string(accountSeed))

	// Create a new set of account claims and configure as desired including a
	// readable name, JetStream limits, imports/exports, etc.
	accountClaims := jwt.NewAccountClaims(accountPub)
	accountClaims.Name = "my-account"

	// The only requirement to "enable" JetStream is setting the disk and memory
	// limits to anything other than zero. -1 indicates "unlimited".
	accountClaims.Limits.JetStreamLimits.DiskStorage = -1
	accountClaims.Limits.JetStreamLimits.MemoryStorage = -1

	// Inspecting the claims, you will notice the `sub` field is the public key
	// of the account.
	fmt.Printf("account claims: %s\n", accountClaims)

	// Now we can sign the claims with the operator and encode it to a JWT string.
	// To activate this account, it must be pushed up to the server using a client
	// connection authenticated as the SYS account user:
	// ```go
	// nc.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(accountJWT))
	// ```
	// If you copy the JWT output to https://jwt.io, you will notice the `iss`
	// field is set to the operator public key.
	accountJWT, _ := accountClaims.Encode(operatorKP)
	fmt.Printf("account jwt: %s\n\n", accountJWT)

	// It is important to call out that the nsc tool handles storage and management
	// of operators, accounts, users. It writes out each nkey and JWT to a file and
	// organizes everything for you. If you opt to create accounts or users dynamically,
	// keep in mind you need to store and manage the keypairs and JWTs yourself.

	// If we want to create a user, the process is essentially the same as it was
	// for the account.
	userKP, _ := nkeys.CreateUser()

	userPub, _ := userKP.PublicKey()
	fmt.Printf("user pubkey: %s\n", userPub)

	userSeed, _ := userKP.Seed()
	fmt.Printf("user seed: %s\n", string(userSeed))

	// Create the user claims, set the name, and configure permissions, expiry time,
	// limits, etc.
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Name = "my-user"

	userClaims.Limits.Data = 1024 * 1024 * 1024

	userClaims.Permissions.Pub.Allow.Add("foo.>", "bar.>")
	userClaims.Permissions.Sub.Allow.Add("_INBOX.>")

	fmt.Printf("userclaims: %s\n", userClaims)

	// Sign and encode the claims as a JWT.
	userJWT, _ := userClaims.Encode(accountKP)
	fmt.Printf("user jwt: %s\n", userJWT)

	creds, _ := jwt.FormatUserConfig(userJWT, userSeed)
	fmt.Printf("creds file: %s\n", creds)
}
