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
			Address: "localhost",
			Port:    &http_port,
		},
	}
	err = f.Driver.InitializeRelay(f.relays["http"])
	if err != nil {
		return err
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
			Address: "localhost",
			Port:    &grpc_port,
			Npn:     "spdy/2",
			Alpn:    "h2,http/1.1",
		},
	}
	err = f.Driver.InitializeRelay(f.relays["grpc"])
	if err != nil {
		return err
	}

	_, front, err := f.Driver.HAProxy.GetConfiguration().GetFrontends("")
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
	logerr(f.weed.ProcessState.String())

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
