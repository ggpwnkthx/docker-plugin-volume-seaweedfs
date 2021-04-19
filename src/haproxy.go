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

func (f *Filer) InitializeRelays() error {
	for _, relay := range f.relays {
		_, _, err := f.Driver.HAProxy.Configuration.GetBackend(relay.Backend.Name, "")
		if err != nil {
			logerr(err.Error())
			version, _ := f.Driver.HAProxy.Configuration.GetVersion("")
			err = f.Driver.HAProxy.Configuration.CreateBackend(relay.Backend, "", version)
			if err != nil {
				return err
			}
		}
		_, _, err = f.Driver.HAProxy.Configuration.GetServer(relay.Server.Name, relay.Backend.Name, "")
		if err != nil {
			logerr(err.Error())
			version, _ := f.Driver.HAProxy.Configuration.GetVersion("")
			err = f.Driver.HAProxy.Configuration.CreateServer(relay.Backend.Name, relay.Server, "", version)
			if err != nil {
				return err
			}
		}
		_, _, err = f.Driver.HAProxy.Configuration.GetFrontend(relay.Frontend.Name, "")
		if err != nil {
			logerr(err.Error())
			version, _ := f.Driver.HAProxy.Configuration.GetVersion("")
			err = f.Driver.HAProxy.Configuration.CreateFrontend(relay.Frontend, "", version)
			if err != nil {
				return err
			}
		}
		_, _, err = f.Driver.HAProxy.Configuration.GetBind(relay.Bind.Name, relay.Frontend.Name, "")
		if err != nil {
			logerr(err.Error())
			version, _ := f.Driver.HAProxy.Configuration.GetVersion("")
			err = f.Driver.HAProxy.Configuration.CreateBind(relay.Frontend.Name, relay.Bind, "", version)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
