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
	"github.com/haproxytech/models"
)

type Filer struct {
	alias      string
	Driver     *Driver
	relays     map[string]*Relay
	Mountpoint string
	weed       *exec.Cmd
}
type Relay struct {
	port   int64
	socket string
}

func (f *Filer) init() error {
	port, err := getFreePort()
	if err != nil {
		return err
	}
	logerr("initializing filer using port", strconv.FormatInt(port, 10))

	f.relays = map[string]*Relay{}
	f.relays["http"] = &Relay{
		port:   port,
		socket: filepath.Join(seaweedfsSockets, f.alias, "http.sock"),
	}
	if _, err := os.Stat(f.relays["http"].socket); os.IsNotExist(err) {
		return errors.New("http unix socket not found")
	}
	f.relays["grpc"] = &Relay{
		port:   port + 10000,
		socket: filepath.Join(seaweedfsSockets, f.alias, "grpc.sock"),
	}
	if _, err := os.Stat(f.relays["grpc"].socket); os.IsNotExist(err) {
		return errors.New("grpc unix socket not found")
	}

	version, backends, err := f.Driver.HAProxy.Configuration.GetBackends("")
	if err != nil {
		return err
	}
	http_found := false
	grpc_found := false
	for _, b := range backends {
		if b.Name == f.alias+"_http" {
			http_found = true
		}
		if b.Name == f.alias+"_grpc" {
			grpc_found = true
		}
	}
	if !http_found {
		err := f.Driver.HAProxy.Configuration.CreateBackend(&models.Backend{
			Name: f.alias + "_http",
			Mode: "http",
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateServer(f.alias+"_http", &models.Server{
			Name:    f.alias + "_http",
			Address: filepath.Join(seaweedfsSockets, f.alias, "http.sock"),
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateFrontend(&models.Frontend{
			Name:           f.alias + "_http",
			Mode:           "http",
			DefaultBackend: f.alias + "_http",
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateBind(f.alias+"_http", &models.Bind{
			Name:    f.alias + "_http",
			Address: "0.0.0.0",
			Port:    &f.relays["https"].port,
		}, "", version)
		if err != nil {
			return err
		}
	}
	if !grpc_found {
		err := f.Driver.HAProxy.Configuration.CreateBackend(&models.Backend{
			Name: f.alias + "_grpc",
			Mode: "tcp",
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateServer(f.alias+"_grpc", &models.Server{
			Name:    f.alias + "_grpc",
			Address: filepath.Join(seaweedfsSockets, f.alias, "grpc.sock"),
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateFrontend(&models.Frontend{
			Name:           f.alias + "_grpc",
			Mode:           "tcp",
			DefaultBackend: f.alias + "_grpc",
		}, "", version)
		if err != nil {
			return err
		}
		err = f.Driver.HAProxy.Configuration.CreateBind(f.alias+"_grpc", &models.Bind{
			Name:    f.alias + "_grpc",
			Address: "0.0.0.0",
			Port:    &f.relays["grpc"].port,
		}, "", version)
		if err != nil {
			return err
		}
	}

	f.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, f.alias)
	os.MkdirAll(f.Mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + f.Mountpoint,
		"-filer=localhost:" + strconv.FormatInt(f.relays["http"].port, 10),
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
