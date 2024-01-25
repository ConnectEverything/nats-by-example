package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
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

	storage, err := os.MkdirTemp(".", "tmp-embedded-store")
	if err != nil {
		log.Fatalf("unable to create temp dir: %v", err)
	}
	defer func() {
		if !(debugEnabled || traceEnabled) {
			os.RemoveAll(storage)
		}
	}()

	//
	// Configure each server
	//

	var natsServerOptions []server.Options
	var routes []*url.URL
	var nodes []*server.Server

	for ix, name := range serverNames {
		natsPort := startingNatsPort + ix
		clusterPort := startingClusterPort + ix

		opt := server.Options{
			ServerName:         name,
			Host:               clusterHost,
			Port:               natsPort,
			JetStream:          jetstreamEnabled,
			JetStreamMaxStore:  1 << 20,
			JetStreamMaxMemory: 1 << 20,
			StoreDir:           storage,
			SystemAccount:      systemAccountName,
			NoAuthUser:         defaultUser,
			Accounts:           accounts,
			Users:              users,
			Debug:              debugEnabled,
			Trace:              traceEnabled,
			NoLog:              !(debugEnabled || traceEnabled),
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

		nodes = append(nodes, ns)

		wg.Add(1)
		go func(s *server.Server) {
			s.WaitForShutdown()
			wg.Done()
		}(ns)
	}

	//
	// Issue a client command to check the state of the cluster.
	//

	// give cluster time to settle; wait for a bit
	<-time.After(10 * time.Second)

	// generate report
	report, err := jetStreamReport(optsHelper(natsServerOptions).ConnectionString(systemUser, systemPass))
	if err != nil {
		log.Fatalf("unable to get server report: %v", err)
	}

	// shutdown cluster
	for _, node := range nodes {
		node.Shutdown()
	}

	// Run until servers are terminated
	wg.Wait()

	// print report
	log.Print(report)
}

func jetStreamReport(connstr string) (string, error) {
	// connect to cluster
	nc, err := nats.Connect(connstr)
	if err != nil {
		return "", err
	}

	// issue synchronous request/reply on API subject
	body := `{"accounts":true,"streams":true,"consumer":true,"raft":true}`
	reply, err := nc.Request("$SYS.REQ.SERVER.PING.JSZ", []byte(body), time.Second*10)
	if err != nil {
		return "", err
	}

	// use helper to format report
	var data jsz
	if err := json.Unmarshal(reply.Data, &data); err != nil {
		return "", err
	}

	return data.String(), nil
}

// jsz is a convenience type to capture jetstream and server status data.
type jsz struct {
	Data   server.JSInfo     `json:"data"`
	Server server.ServerInfo `json:"server"`
}

func (z jsz) String() string {
	str := fmt.Sprintf(`
            SERVER: %s [%s]
           VERSION: %s
           CLUSTER: %s
              HOST: %s

 JetStream Enabled: %t
        Max Memory: %d (bytes)
          Max Disk: %d (bytes)
`,
		z.Server.Name,
		z.Server.ID,
		z.Server.Version,
		z.Server.Cluster,
		z.Server.Host,
		z.Server.JetStream,
		z.Data.Config.MaxMemory,
		z.Data.Config.MaxStore,
	)

	if z.Data.Meta != nil {
		str = fmt.Sprintf(`
%s

    Cluster Leader: %s
      Cluster Size: %d
`,
			str,
			z.Data.Meta.Leader,
			z.Data.Meta.Size,
		)
	}

	return str
}

// optsHelper is a convenience type to represent a slice of server.Options as a connection string for NATS.
type optsHelper []server.Options

// ConnectionString builds a connection string with the provided username and password.
func (h optsHelper) ConnectionString(user, pass string) string {
	var urls []string
	for _, o := range h {
		urls = append(urls, fmt.Sprintf("nats://%s:%s@%s:%d", user, pass, o.Host, o.Port))
	}
	return strings.Join(urls, ",")
}
