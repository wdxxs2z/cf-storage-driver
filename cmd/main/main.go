package main

import (
	"flag"
	"os"

	cf_lager "code.cloudfoundry.org/cflager"
	cf_debug_server "code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/ifrit"
	//"github.com/wdxxs2z/cf-storage-driver/storage_server"
	"../../storage_server"
)

func parseConfig(config *storage_server.DriverServerConfig) {

	flag.StringVar(&config.ListenAddress,"listenAddress","0.0.0.0:5566","host:port nfsdriver manager listen address")
	flag.StringVar(&config.DriversPath, "driversPath", "/tmp/voldriver", "nfs driver path where the voldriver installed")
	flag.StringVar(&config.Transport, "transport", "tcp", "tcp or unix transport protocol,default tcp")
	flag.StringVar(&config.RegistryDriver, "registryDriver", "nfs", "support storage backend driver,now available drivers are nfs,local...")

	cf_lager.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)

	flag.Parse()
}

func init() {}

func main() {
	storageConfig := storage_server.DriverServerConfig{}

	parseConfig(&storageConfig)

	storageLogger, _ := cf_lager.New("storage-driver-server")

	storageServer := storage_server.NewStorageDriverServer(storageConfig)

	storageDriverServer, err := storageServer.Runner(storageLogger)

	exitOnFailure(storageLogger, err)

	servers := grouper.Members{
		{"storage-driver-server", storageDriverServer},
	}

	var logTap *lager.ReconfigurableSink

	if degugAddr := cf_debug_server.DebugAddress(flag.CommandLine); degugAddr != "" {
		servers = append(grouper.Members{{"storage-driver-debug-server", cf_debug_server.Runner(degugAddr, logTap)}}, servers...)
	}

	runner := sigmon.New(grouper.NewOrdered(os.Interrupt,servers))
	process := ifrit.Invoke(runner)
	storageLogger.Info("started")
	untilTerminated(storageLogger, process)

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
