package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-plugins-helpers/volume"
)

type Filer struct {
	alias      string
	Driver     *Driver
	relays     map[string]*Relay
	Mountpoint string
	weed       *exec.Cmd
}
type Relay struct {
	port   int
	socket string
	c1     *net.Conn
	c2     *net.Conn
}

func (f *Filer) init() error {
	port, err := getFreePort()
	if err != nil {
		return err
	}
	logerr("initializing filer using port", strconv.Itoa(port))

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

	go f.proxet(f.relays["http"])
	go f.proxet(f.relays["grpc"])

	f.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, f.alias)
	os.MkdirAll(f.Mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + f.Mountpoint,
		"-filer=localhost:" + strconv.Itoa(f.relays["http"].port),
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

func (f *Filer) proxet(relay *Relay) {
	listener, err := net.Listen("tcp", "localhost:"+strconv.Itoa(relay.port))
	if err != nil {
		logerr(err.Error())
	}
	for {
		c1, err := listener.Accept()
		relay.c1 = &c1
		if err != nil {
			logerr(err.Error())
			continue
		}
		go f.proxetHandler(relay)
	}
}
func (f *Filer) proxetHandler(relay *Relay) {
	c2, err := net.Dial("unix", relay.socket)
	if err != nil {
		logerr(err.Error())
		return
	}
	relay.c2 = &c2
	go proxetCopy(relay.c1, relay.c2) // c1 -> c2
	proxetCopy(relay.c2, relay.c1)    // c2 -> c1
}
func proxetCopy(writer *net.Conn, reader *net.Conn) {
	_, err := io.Copy(*writer, *reader)
	if err != nil {
		logerr(err.Error())
	}
}
