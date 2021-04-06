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
	alias  string
	Driver *Driver
	relays map[string]*Relay
	weed   *exec.Cmd
}
type Relay struct {
	port     int
	socket   string
	listener *net.Listener
	c1       *net.Conn
	c2       *net.Conn
}

func (f *Filer) init() error {
	port, err := getFreePort()
	if err != nil {
		return err
	}
	logerr("initializing filer using port " + strconv.Itoa(port))

	f.relays = map[string]*Relay{}
	f.relays["http"].port = port
	f.relays["http"].socket = filepath.Join(seaweedfsSockets, f.alias, "http.sock")
	if _, err := os.Stat(f.relays["http"].socket); os.IsNotExist(err) {
		return errors.New("http unix socket not found")
	}
	f.relays["grpc"].port = port + 10000
	f.relays["grpc"].socket = filepath.Join(seaweedfsSockets, f.alias, "grpc.sock")
	if _, err := os.Stat(f.relays["grpc"].socket); os.IsNotExist(err) {
		return errors.New("grpc unix socket not found")
	}
	listener, err := net.Listen("tcp", "localhost:"+strconv.Itoa(f.relays["http"].port))
	if err != nil {
		logerr(err.Error())
	}
	f.relays["http"].listener = &listener
	listener, err = net.Listen("tcp", "localhost:"+strconv.Itoa(f.relays["grpc"].port))
	if err != nil {
		logerr(err.Error())
	}
	f.relays["grpc"].listener = &listener

	go f.proxet("http")
	go f.proxet("grpc")

	mountpoint := filepath.Join("/mnt", f.alias)
	os.MkdirAll(mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-dir=" + mountpoint,
		"-filer=localhost:" + strconv.Itoa(f.relays["http"].port),
		"-volumeServerAccess=filerProxy",
	}
	SeaweedFSMount(f.weed, mOptions)
	f.Driver.Filers[f.alias] = f

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
			if r.Options == nil {
				r.Options = map[string]string{}
			}
			r.Options["filer"] = f.alias
			v := new(Volume)
			err := v.Create(&r, f.Driver)
			if err != nil {
				logerr(err.Error())
			}
			names = append(names, r.Name)
		}
		for name, volume := range driver.Volumes {
			if !Contains(names, name) {
				logerr("found unused mount " + name)
				if volume.Filer.alias == f.alias {
					logerr("removing mount " + name)
					delete(driver.Volumes, name)
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

func (f *Filer) proxet(handle string) {
	for {
		if f.relays[handle] == nil {
			break
		}
		c1, err := (*f.relays[handle].listener).Accept()
		f.relays[handle].c1 = &c1
		if err != nil {
			logerr(err.Error())
			continue
		}
		go f.proxetHandler(handle)
	}
}
func (f *Filer) proxetHandler(handle string) {
	c2, err := net.Dial("unix", f.relays[handle].socket)
	if err != nil {
		logerr(err.Error())
		return
	}
	f.relays[handle].c2 = &c2
	go proxetCopy(f.relays[handle].c1, f.relays[handle].c2) // c1 -> c2
	proxetCopy(f.relays[handle].c2, f.relays[handle].c1)    // c2 -> c1
}
func proxetCopy(writer *net.Conn, reader *net.Conn) {
	_, err := io.Copy(*writer, *reader)
	if err != nil {
		logerr(err.Error())
	}
}
