package main

import (
	"strconv"

	client_native "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	runtime_api "github.com/haproxytech/client-native/v2/runtime"
	models "github.com/haproxytech/models/v2"
)

type Relay struct {
	Backend  *models.Backend
	Server   *models.Server
	Frontend *models.Frontend
	Bind     *models.Bind
}

func (d *Driver) ConfigureHAProxy() error {
	hapcc := &configuration.Client{}
	confParams := configuration.ClientParams{
		ConfigurationFile:      "/etc/haproxy/haproxy.cfg",
		Haproxy:                "/usr/sbin/haproxy",
		UseValidation:          true,
		PersistentTransactions: true,
		TransactionDir:         "/tmp/haproxy/transactions",
	}
	err := hapcc.Init(confParams)
	if err != nil {
		logerr("Error setting up default configuration client, exiting...", err.Error())
	}
	haprtc := &runtime_api.Client{}
	version, globalConf, err := hapcc.GetGlobalConfiguration("")
	logerr("GetGlobalConfiguration:", "version", strconv.FormatInt(version, 10))
	if err == nil {
		socketList := map[int]string{}
		runtimeAPIs := globalConf.RuntimeAPIs

		if len(runtimeAPIs) != 0 {
			for i, r := range runtimeAPIs {
				socketList[i] = *r.Address
			}
			if err := haprtc.InitWithSockets(socketList); err != nil {
				logerr("Error setting up runtime client, not using one")
				return nil
			}
		} else {
			logerr("Runtime API not configured, not using it")
			haprtc = nil
		}
	} else {
		logerr("Cannot read runtime API configuration, not using it")
		haprtc = nil
	}

	d.HAProxy = &client_native.HAProxyClient{}
	err = d.HAProxy.Init(hapcc, haprtc)
	return err
}

func (d *Driver) InitializeRelay(relay *Relay) error {
	config := d.HAProxy.GetConfiguration()
	_, _, err := config.GetBackend(relay.Backend.Name, "")
	if err != nil {
		logerr(err.Error())
		version, _ := config.GetVersion("")
		err = config.CreateBackend(relay.Backend, "", version)
		if err != nil {
			return err
		}
		logerr("created backend", relay.Backend.Name)
	}
	_, _, err = config.GetServer(relay.Server.Name, relay.Backend.Name, "")
	if err != nil {
		logerr(err.Error())
		version, _ := config.GetVersion("")
		err = config.CreateServer(relay.Backend.Name, relay.Server, "", version)
		if err != nil {
			return err
		}
		logerr("created server", relay.Server.Name)
	}
	_, _, err = config.GetFrontend(relay.Frontend.Name, "")
	if err != nil {
		logerr(err.Error())
		version, _ := config.GetVersion("")
		err = config.CreateFrontend(relay.Frontend, "", version)
		if err != nil {
			return err
		}
		logerr("created frontend", relay.Frontend.Name)
	}
	_, _, err = config.GetBind(relay.Bind.Name, relay.Frontend.Name, "")
	if err != nil {
		logerr(err.Error())
		version, _ := config.GetVersion("")
		err = config.CreateBind(relay.Frontend.Name, relay.Bind, "", version)
		if err != nil {
			return err
		}
		logerr("created bind", relay.Bind.Name)
	}
	return nil
}
