# Create JWTs Programmatically

The primary (and recommended) way to create and manage accounts and users is using the [nsc](https://nats-io.github.io/nsc/) command-line tool. However, in some applications and use cases, it may be desirable to programmatically create accounts or users on-demand as part of an application-level account/user workflow rather than out-of-band on the command line (however, shelling out to `nsc` from your program is another option).

This program implements two basic functions, one for creating an account and one for creating a user.

## Usage

Either the operator seed is required for creating accounts or the account seed is required for creating users. To view the seeds via `nsc` you can use:

```
nsc list keys --show-seeds
```

If using signing keys, ensure you choose that seed.

To create an account, specify the `-operator` supplying the seed and `-name` for the name of the account.

```
go run main.go -operator <seed> -name <account>
```

Similarly, create a user by providing the account seed.

```sh
go run main.go -account <seed> -name <user>
```
