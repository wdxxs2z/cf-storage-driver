package storage_server

import (
	"strings"
	"fmt"
	"encoding/json"

	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	//"github.com/wdxxs2z/cf-storage-driver/storage_local/nfs"
	"../storage_local/nfs"
	"../storage_local/local"

	"net/http"
)

type DriverServerConfig struct {
	ListenAddress    string
	DriversPath      string
	Transport        string
	RegistryDriver   string
	MountDir         string
}

type DriverServer struct  {
	config DriverServerConfig
}

type StorageDriverServer interface {
	Runner(logger lager.Logger) (ifrit.Runner, error)
}

func NewStorageDriverServer(storageConfig DriverServerConfig) StorageDriverServer {
	return &DriverServer{
		config:        storageConfig,
	}
}

func (server *DriverServer) Runner(logger lager.Logger) (ifrit.Runner, error) {
	var err error
	var storageDriverServer ifrit.Runner

	server.config.Transport = server.DetermineTransport(server.config.ListenAddress)
	if server.config.Transport == "tcp" {
		storageDriverServer, err = server.CreateTcpServer(logger, server.config.ListenAddress, server.config.DriversPath)
	} else {
		storageDriverServer, err = server.CreateUnixServer(logger, server.config.ListenAddress, server.config.DriversPath)
	}
	if err != nil {
		return nil, err
	}
	return storageDriverServer, nil
}

func (server *DriverServer) CreateTcpServer(logger lager.Logger, address string, driversPath string) (ifrit.Runner, error) {
	logger.Session("create-tcp-server")
	logger.Info("start")
	defer logger.Info("end")

	driverName := server.config.RegistryDriver
	var handler http.Handler
	var err error

	switch driverName {
	case "nfs":
		handler,err = server.createHttpHandler(logger, address, driverName, driversPath, "tcp", storage_nfsdriver.NewNfsLocalDriver())
	case "local":
		server.createHttpHandler(logger, address, driverName, driversPath, "tcp", storage_localdriver.NewLocalDriver(server.config.MountDir))
	}

	if err != nil {
		return nil, err
	}
	return http_server.New(address, handler), nil
}

func (server *DriverServer) CreateUnixServer(logger lager.Logger, address string, driversPath string) (ifrit.Runner, error) {
	logger.Session("create-unix-server")
	logger.Info("start")
	defer logger.Info("end")

	driverName := server.config.RegistryDriver
	var handler http.Handler
	var err error

	switch driverName {
	case "nfs":
		handler,err = server.createHttpHandler(logger, address, driverName, driversPath, "unix",storage_nfsdriver.NewNfsLocalDriver())
	case "local":
		server.createHttpHandler(logger, address, driverName, driversPath, "unix", storage_localdriver.NewLocalDriver(server.config.MountDir))
	}

	if err != nil {
		return nil, err
	}
	return http_server.NewUnixServer(address, handler), nil
}

func (server *DriverServer) createHttpHandler(logger lager.Logger, address,driver,driversPath,mode string, client voldriver.Driver) (http.Handler, error){
	driverName := fmt.Sprintf("%sdriver", driver)
	logger.Session(fmt.Sprintf("create-%s-driver-spec",driver))
	logger.Info("start")
	defer logger.Info("end")

	switch mode {
	case "tcp":
		spec := voldriver.DriverSpec{
			Name:             driverName,
			Address:          server.rewriteAddress(address, "http"),
		}

		specJson, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}

		err = voldriver.WriteDriverSpec(logger, driversPath, driverName, "json", specJson)
		if err != nil {
			return nil, err
		}
	case "unix":
		url := server.rewriteAddress(address, "unix")
		err := voldriver.WriteDriverSpec(logger, driversPath, "nfsdriver", "spec", []byte(url))
		if err != nil {
			return nil, err
		}
	}
	return driverhttp.NewHandler(logger, client)
}

func (server *DriverServer) rewriteAddress(address string, protocol string) string {
	if !strings.HasPrefix(address, protocol + "://") {
		return fmt.Sprintf("%s://%s", protocol, address)
	}
	return address
}

func (server *DriverServer) DetermineTransport(address string) string {
	if strings.HasSuffix(address, ".sock") {
		return "unix"
	}
	return "tcp"
}
