package main

import (
	"flag"
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/nats-io/nats-server/v2/server"
)

func main() {
	//
	// Setup flags and defaults
	//

	var (
		clusterHost         string
		startingNatsPort    int
		startingClusterPort int
		clusterName         string
		systemAccountName   string
		defaultAccountName  string
		jetstreamEnabled    bool
		debugEnabled        bool
		traceEnabled        bool
		systemUser          string
		systemPass          string
		defaultUser         string
		defaultPass         string
	)

	flag.StringVar(&clusterHost, "host", "localhost", "Host to bind to.")
	flag.IntVar(&startingNatsPort, "port", 4222, "First NATS port (each additional server will increment by one)")
	flag.IntVar(&startingClusterPort, "cluster.port", 6222, "First cluster port (each additional server will increment by one)")
	flag.StringVar(&clusterName, "cluster.name", "embedded-cluster", "Name of NATS cluster.")
	flag.StringVar(&systemAccountName, "system.account", "$SYS", "Name of system account.")
	flag.StringVar(&defaultAccountName, "default.account", "DEFAULT", "Name of default account.")
	flag.BoolVar(&jetstreamEnabled, "js", true, "JetStream enabled.")
	flag.BoolVar(&debugEnabled, "debug", false, "Debug logging enabled.")
	flag.BoolVar(&traceEnabled, "trace", false, "Trace logging enabled.")
	flag.StringVar(&systemUser, "system.user", "admin", "System account user.")
	flag.StringVar(&systemPass, "system.pass", "admin", "System account password.")
	flag.StringVar(&defaultUser, "default.user", "default", "Default account user.")
	flag.StringVar(&defaultPass, "default.pass", "default", "Default account password.")

	flag.Parse()

	serverNames := flag.Args()
	if len(serverNames) == 0 {
		serverNames = []string{"nats-0", "nats-1", "nats-2"}
	}

	sort.Strings(serverNames)
	cluster := len(serverNames) > 1

	systemAccount := server.NewAccount(systemAccountName)
	defaultAccount := server.NewAccount(defaultAccountName)
	accounts := []*server.Account{systemAccount, defaultAccount}
	users := []*server.User{
		{Username: systemUser, Password: systemPass, Account: systemAccount},
		{Username: defaultUser, Password: defaultPass, Account: defaultAccount},
	}

	//
	// Configure each server
	//

	natsServerOptions := []server.Options{}
	routes := []*url.URL{}

	for ix, name := range serverNames {
		natsPort := startingNatsPort + ix
		clusterPort := startingClusterPort + ix

		opt := server.Options{
			ServerName:    name,
			Host:          clusterHost,
			Port:          natsPort,
			JetStream:     jetstreamEnabled,
			SystemAccount: systemAccountName,
			NoAuthUser:    defaultUser,
			Accounts:      accounts,
			Users:         users,
			Debug:         debugEnabled,
			Trace:         traceEnabled,
		}

		if cluster {
			opt.Cluster = server.ClusterOpts{
				Name: clusterName,
				Port: clusterPort,
				Host: clusterHost,
			}
		}

		natsServerOptions = append(natsServerOptions, opt)
		routes = append(routes, &url.URL{Scheme: "nats", Host: fmt.Sprintf("%s:%d", clusterHost, clusterPort)})
	}

	//
	// Start each server
	//

	var wg sync.WaitGroup
	for _, loopOpt := range natsServerOptions {
		localOpt := loopOpt
		localOpt.Routes = routes

		ns, err := server.NewServer(&localOpt)
		if err != nil {
			server.PrintAndDie(fmt.Sprintf("initializing server %q: %v", localOpt.ServerName, err))
		}

		ns.ConfigureLogger()
		ns.Start()

		wg.Add(1)
		go func(s *server.Server) {
			s.WaitForShutdown()
			wg.Done()
		}(ns)
	}

	//
	// Run until servers are terminated
	//

	wg.Wait()
}
