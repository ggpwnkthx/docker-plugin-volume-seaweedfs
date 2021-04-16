package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-plugins-helpers/volume"
	models "github.com/haproxytech/models/v2"
)

type Filer struct {
	alias      string
	Driver     *Driver
	relays     map[string]*Relay
	Mountpoint string
	weed       *exec.Cmd
}
type Relay struct {
	Backend  *models.Backend
	Server   *models.Server
	Frontend *models.Frontend
	Bind     *models.Bind
}

func (f *Filer) init() error {
	http_port, err := getFreePort()
	if err != nil {
		return err
	}
	grpc_port := http_port + 10000
	logerr("initializing filer using port", strconv.FormatInt(http_port, 10))

	f.relays = map[string]*Relay{}
	f.relays["http"] = &Relay{
		Backend: &models.Backend{
			Name: f.alias + "_http",
			Mode: "http",
		},
		Server: &models.Server{
			Name:    f.alias + "_http",
			Address: filepath.Join(seaweedfsSockets, f.alias, "http.sock"),
		},
		Frontend: &models.Frontend{
			Name:           f.alias + "_http",
			Mode:           "http",
			DefaultBackend: f.alias + "_http",
		},
		Bind: &models.Bind{
			Name:    f.alias + "_http",
			Address: "0.0.0.0",
			Port:    &http_port,
		},
	}

	f.relays["grpc"] = &Relay{
		Backend: &models.Backend{
			Name: f.alias + "_grpc",
			Mode: "tcp",
		},
		Server: &models.Server{
			Name:    f.alias + "_grpc",
			Address: filepath.Join(seaweedfsSockets, f.alias, "grpc.sock"),
		},
		Frontend: &models.Frontend{
			Name:           f.alias + "_grpc",
			Mode:           "tcp",
			DefaultBackend: f.alias + "_grpc",
		},
		Bind: &models.Bind{
			Name:    f.alias + "_grpc",
			Address: "0.0.0.0",
			Port:    &grpc_port,
		},
	}

	err = f.InitializeRelays()
	if err != nil {
		return err
	}
	_, front, err := f.Driver.HAProxy.Configuration.GetFrontends("")
	if err != nil {
		return err
	}
	frontJSON, err := json.Marshal(front)
	if err != nil {
		return err
	}
	logerr(string(frontJSON))

	f.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, f.alias)
	os.MkdirAll(f.Mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + f.Mountpoint,
		"-filer=localhost:" + strconv.FormatInt(*f.relays["http"].Bind.Port, 10),
		"-volumeServerAccess=filerProxy",
	}
	f.weed = SeaweedFSMount(mOptions)

	f.Driver.addFiler(f)
	logerr("filer", f.alias, "initialized")

	return nil
}
func (f *Filer) load(alias string, driver *Driver) error {
	if !isFiler(alias) {
		return errors.New("filer " + alias + " does not exist")
	} else {
		logerr("loading filer", alias)
	}
	f.alias = alias
	f.Driver = driver
	if !f.Driver.isFiler(f.alias) {
		err := f.init()
		if err != nil {
			return err
		}
	}

	path := filepath.Join(f.Mountpoint, "volumes.json")
	data, err := ioutil.ReadFile(path)
	if err == nil {
		logerr("found ", path)
		requests := []volume.CreateRequest{}
		names := []string{}
		json.Unmarshal(data, &requests)
		for _, r := range requests {
			if r.Options == nil {
				r.Options = map[string]string{}
			}
			r.Options["filer"] = f.alias
			logerr("adding volume", r.Name, "from filer", f.alias)
			v := new(Volume)
			err := v.Create(&r, f.Driver)
			if err != nil {
				logerr(err.Error())
			}
			names = append(names, r.Name)
		}
		for name, volume := range driver.Volumes {
			if !Contains(names, name) {
				logerr("found unused mount", name)
				if volume.Filer.alias == f.alias {
					f.Driver.deleteVolume(volume)
				}
			}
		}
	}
	return nil
}
func (f *Filer) saveRunning() error {
	volumes := []*volume.CreateRequest{}
	for _, v := range f.Driver.Volumes {
		if v.Options["filer"] == f.alias {
			logerr("saveRunning:", "found volume", v.Name)
			volume := volume.CreateRequest{
				Name:    v.Name,
				Options: v.Options,
			}
			volumes = append(volumes, &volume)
		}
	}
	return f.save(volumes)
}
func (f *Filer) save(volumes []*volume.CreateRequest) error {
	logerr("saving volumes on", f.alias)
	for _, v := range volumes {
		delete(v.Options, "filer")
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		return err
	}
	path := filepath.Join(f.Mountpoint, "volumes.json")
	logerr("saving to", path)
	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
func (f *Filer) InitializeRelays() error {
	version, _ := f.Driver.HAProxy.Configuration.GetVersion("")
	for _, relay := range f.relays {
		err := f.Driver.HAProxy.Configuration.CreateBackend(relay.Backend, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateServer(relay.Backend.Name, relay.Server, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateFrontend(relay.Frontend, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateBind(relay.Frontend.Name, relay.Bind, "", version)
		if err != nil {
			return err
		}
	}
	return nil
}

func isFiler(alias string) bool {
	http := filepath.Join(seaweedfsSockets, alias, "http.sock")
	if _, err := os.Stat(http); os.IsNotExist(err) {
		logerr("isFiler:", alias, "is missing http.sock")
		return false
	}
	grpc := filepath.Join(seaweedfsSockets, alias, "grpc.sock")
	if _, err := os.Stat(grpc); os.IsNotExist(err) {
		logerr("isFiler:", alias, "is missing grpc.sock")
		return false
	}
	logerr("isFiler:", alias, "is a filer")
	return true
}
func availableFilers() ([]string, error) {
	dirs := []string{}
	items, err := ioutil.ReadDir(seaweedfsSockets)
	if err != nil {
		return []string{}, err
	}
	for _, i := range items {
		if i.IsDir() {
			logerr("availableFilers:", "found dir", i.Name())
			dirs = append(dirs, i.Name())
		}
	}
	filers := []string{}
	for _, d := range dirs {
		if isFiler(d) {
			filers = append(filers, d)
		}
	}
	return filers, nil
}
