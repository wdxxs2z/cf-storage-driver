package nfsserver

import (
	"strings"
	"fmt"
	"encoding/json"

	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/wdxxs2z/nfsdriver-init/storage_local/nfs"

)

type DriverServerConfig struct {
	AtAddress    string
	DriversPath  string
	Transport    string
}

type DriverServer struct  {
	config DriverServerConfig
}

type NfsDriverServer interface {
	Runner(logger lager.Logger) (ifrit.Runner, error)
}

func NewNfsDriverServer(nfsConfig DriverServerConfig) NfsDriverServer {
	return &DriverServer{
		config:        nfsConfig,
	}
}

func (server *DriverServer) Runner(logger lager.Logger) (ifrit.Runner, error) {
	var err error
	var nfsDriverServer ifrit.Runner

	server.config.Transport = server.DetermineTransport(server.config.AtAddress)
	if server.config.Transport == "tcp" {
		nfsDriverServer, err = server.CreateTcpServer(logger, server.config.AtAddress, server.config.DriversPath)
	} else {
		nfsDriverServer, err = server.CreateUnixServer(logger, server.config.AtAddress, server.config.DriversPath)
	}
	if err != nil {
		return nil, err
	}
	return nfsDriverServer, nil
}

func (server *DriverServer) CreateTcpServer(logger lager.Logger, address string, driversPath string) (ifrit.Runner, error) {
	logger.Session("create-tcp-server")
	logger.Info("start")
	defer logger.Info("end")

	//validation tcp address

	spec := voldriver.DriverSpec{
		Name:             "nfsdriver",
		Address:           server.rewriteAddress(address, "http"),
	}
	specJson, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	err = voldriver.WriteDriverSpec(logger, driversPath, "nfsdriver", "json", specJson)
	if err != nil {
		return nil, err
	}
	handler, err := driverhttp.NewHandler(logger, nfslocal.NewLocalDriver())
	if err != nil {
		return nil, err
	}
	return http_server.New(address, handler), nil
}

func (server *DriverServer) rewriteAddress(address string, protocol string) string {
	if !strings.HasPrefix(address, protocol + "://") {
		return fmt.Sprintf("%s://%s", protocol, address)
	}
	return address
}

func (server *DriverServer) CreateUnixServer(logger lager.Logger, address string, driversPath string) (ifrit.Runner, error) {
	logger.Session("create-unix-server")
	logger.Info("start")
	defer logger.Info("end")

	//validate unix protocol

	url := server.rewriteAddress(address, "unix")
	err := voldriver.WriteDriverSpec(logger, driversPath, "nfsdriver", "spec", []byte(url))
	if err != nil {
		return nil, err
	}
	handler, err := driverhttp.NewHandler(logger, nfslocal.NewLocalDriver())
	if err != nil {
		return nil, err
	}
	return http_server.NewUnixServer(address, handler), nil
}

func (server *DriverServer) DetermineTransport(address string) string {
	if strings.HasSuffix(address, ".sock") {
		return "unix"
	}
	return "tcp"
}