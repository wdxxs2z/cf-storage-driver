package main

import (
	"flag"
	"github.com/wdxxs2z/nfsdriver-init/nfsserver"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/ifrit"
	"os"

)

func parseConfig(config *nfsserver.DriverServerConfig) {
	flag.StringVar(&config.AtAddress,"atAddress","0.0.0.0:5566","host:port nfsdriver manager listen address")
	flag.StringVar(&config.DriversPath, "driversPath", "d:\\", "nfs driver path where the voldriver installed")
	flag.StringVar(&config.Transport, "transport", "tcp", "tcp or unix transport protocol,default tcp")

	cf_lager.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)

	flag.Parse()
}

func init() {}

func main() {
	nfsConfig := nfsserver.DriverServerConfig{}
	parseConfig(&nfsConfig)

	logger, reconfigurableSink := cf_lager.New("local-driver-server")

	nfsServer := nfsserver.NewNfsDriverServer(nfsConfig)
	nfsDriverServer, err := nfsServer.Runner(logger)
	exitOnFailure(logger, err)

	servers := grouper.Members{
		{"nfsdriver-server", nfsDriverServer},
	}

	if degug := cf_debug_server.DebugAddress(flag.CommandLine); degug != "" {
		servers = append(grouper.Members{{"nfsdriver-debug-server", cf_debug_server.Runner(degug, reconfigurableSink)}}, servers...)
	}

	runner := sigmon.New(grouper.NewOrdered(os.Interrupt,servers))
	process := ifrit.Invoke(runner)
	logger.Info("started")
	untilTerminated(logger, process)

}

func untilTerminated(logger lager.Logger, process ifrit.Process) {
	err := <-process.Wait()
	exitOnFailure(logger, err)
}

func exitOnFailure(logger lager.Logger, err error) {
	if err != nil {
		logger.Error("fatal-err..aborting", err)
		panic(err.Error())
	}
}