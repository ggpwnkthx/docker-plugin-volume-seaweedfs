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
)

type Filer struct {
	alias  string
	Driver *Driver
	http   struct {
		Port int
		Sock string
	}
	grpc struct {
		Port int
		Sock string
	}
	weed *exec.Cmd
}

func (f *Filer) init() error {
	port, err := getFreePort()
	if err != nil {
		return err
	}
	logerr("using port " + strconv.Itoa(port))

	f.http = struct {
		Port int
		Sock string
	}{
		Port: port,
		Sock: filepath.Join(seaweedfsSockets, f.alias, "http.sock"),
	}
	f.grpc = struct {
		Port int
		Sock string
	}{
		Port: port + 10000,
		Sock: filepath.Join(seaweedfsSockets, f.alias, "grpc.sock"),
	}

	if _, err := os.Stat(f.http.Sock); os.IsNotExist(err) {
		return errors.New("http unix socket not found")
	}
	if _, err := os.Stat(f.grpc.Sock); os.IsNotExist(err) {
		return errors.New("grpc unix socket not found")
	}

	go gocat_tcp2unix(f.http.Port, f.http.Sock)
	go gocat_tcp2unix(f.grpc.Port, f.grpc.Sock)

	mountpoint := filepath.Join("/mnt", f.alias)
	os.MkdirAll(mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-dir=" + mountpoint,
		"-filer=localhost:" + strconv.Itoa(f.http.Port),
		"-volumeServerAccess=filerProxy",
	}
	SeaweedFSMount(f.weed, mOptions)

	return nil
}
func (f *Filer) load(alias string, driver *Driver) error {
	if !isFiler(alias) {
		return errors.New("filer " + alias + " does not exist")
	} else {
		logerr("loading filer " + alias)
	}
	f.alias = alias
	f.Driver = driver
	if _, found := driver.Filers[alias]; !found {
		err := f.init()
		if err != nil {
			return err
		}
	}

	path := filepath.Join("/mnt", f.alias, "volumes.json")
	data, err := ioutil.ReadFile(path)
	if err == nil {
		requests := []volume.CreateRequest{}
		names := []string{}
		json.Unmarshal(data, &requests)
		for _, r := range requests {
			v := new(Volume)
			v.Create(&r, driver)
			logerr("creating mount " + r.Name)
			names = append(names, r.Name)
		}
		for name := range driver.Volumes {
			if !Contains(names, name) {
				if driver.Volumes[name].Filer.alias == f.alias {
					logerr("removing mount " + name)
					delete(driver.Volumes, name)
				}
			}
		}
	}

	driver.Filers[alias] = f
	return nil
}
func (f *Filer) saveRunning() error {
	volumes := []*volume.CreateRequest{}
	for _, v := range f.Driver.Volumes {
		if v.Options["filer"] == f.alias {
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
	logerr("saving volumes on " + f.alias)
	for _, v := range volumes {
		delete(v.Options, "filer")
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		return err
	}
	path := filepath.Join("/mnt", f.alias, "volumes.json")
	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func isFiler(alias string) bool {
	http := filepath.Join(seaweedfsSockets, alias, "http.sock")
	if _, err := os.Stat(http); os.IsNotExist(err) {
		return false
	}
	grpc := filepath.Join(seaweedfsSockets, alias, "grpc.sock")
	if _, err := os.Stat(grpc); os.IsNotExist(err) {
		return false
	}
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
