package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
)

type Driver struct {
	sync.RWMutex
	filers  map[string]*Filer
	sockets string
	Stderr  *os.File
	Stdout  *os.File
	volumes map[string]Volume
}

type Filer struct {
	http *Socat
	grpc *Socat
}

type Socat struct {
	Cmd  *exec.Cmd
	Port int
	Sock string
}

func (d *Driver) load(socketsPath string) {
	d.Lock()
	d.filers = make(map[string]*Filer)
	d.sockets = socketsPath
	d.Stdout = os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout")
	d.Stderr = os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr")
	d.volumes = make(map[string]Volume)
	d.Unlock()

	if _, err := os.Stat(d.sockets + "/volumes.json"); err == nil {
		data, err := ioutil.ReadFile(d.sockets + "/volumes.json")
		if err != nil {
			logrus.WithField("Driver.load()", d.sockets+"/volumes.json").Error(err)
		}
		json.Unmarshal(data, &d.volumes)
		for _, v := range d.volumes {
			v.Update()
		}
	}
}
func (d *Driver) save() error {
	var volumes []Volume
	d.RLock()
	defer d.RUnlock()
	for _, v := range d.volumes {
		volumes = append(volumes, Volume{
			Name:    v.Name,
			Options: v.Options,
		})
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(d.sockets+"/volumes.json", data, 0644); err != nil {
		return err
	}
	return nil
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	v := new(Volume)
	return v.Create(d, r)
}
func (d *Driver) listVolumes() []*volume.Volume {
	d.RLock()
	defer d.RUnlock()
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Status:     v.getStatus(),
		})
	}
	return volumes
}
func (d *Driver) updateVolume(v Volume) error {
	d.Lock()
	defer d.Unlock()
	if v.Mountpoint != "" {
		if d.volumes == nil {
			return errors.New("volumes map not initialized")
		}
		d.volumes[v.Name] = v
	} else {
		delete(d.volumes, v.Name)
	}
	d.save()
	return nil
}

/*
func (d *Driver) manage() {
	for {
		syncState := false
		if _, err := os.Stat(d.sockets + "/volumes.json"); err == nil {
			data, err := ioutil.ReadFile(d.sockets + "/volumes.json")
			if err != nil {
				logrus.WithField("loadDriver", d.sockets+"/volumes.json").Error(err)
			}
			var volumes []Volume
			json.Unmarshal(data, &volumes)

			for _, v := range volumes {
				d.RLock()
				vol := d.volumes[v.Name]
				d.RUnlock()
				if vol == nil {
					v.Update()
					syncState = true
				}
			}
			if syncState {
				d.save()
			}
		}
		time.Sleep(5 * time.Second)
	}
}
*/
func (d *Driver) getFiler(alias string) (*Filer, error) {
	d.RLock()
	_, ok := d.filers[alias]
	d.RUnlock()
	if !ok {
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, alias), os.ModeDir)
		port := 0
		for {
			port, err := freeport.GetFreePort()
			if err != nil {
				return &Filer{}, errors.New("freeport: " + err.Error())
			}
			if port < 55535 {
				break
			}
		}

		socats := &Filer{
			http: &Socat{
				Port: port,
				Sock: filepath.Join(d.sockets, alias, "http.sock"),
			},
			grpc: &Socat{
				Port: port + 10000,
				Sock: filepath.Join(d.sockets, alias, "grpc.sock"),
			},
		}
		if _, err := os.Stat(socats.http.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("http unix socket not found")
		}
		if _, err := os.Stat(socats.grpc.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("grpc unix socket not found")
		}

		httpOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(socats.http.Port) + ",fork",
			"unix:" + socats.http.Sock,
		}
		socats.http.Cmd = exec.Command("/usr/bin/socat", httpOptions...)
		socats.http.Cmd.Stderr = d.Stderr
		socats.http.Cmd.Stdout = d.Stdout
		socats.http.Cmd.Start()

		grpcOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(socats.grpc.Port) + ",fork",
			"unix:" + socats.grpc.Sock,
		}
		socats.grpc.Cmd = exec.Command("/usr/bin/socat", grpcOptions...)
		socats.grpc.Cmd.Stderr = d.Stderr
		socats.grpc.Cmd.Stdout = d.Stdout
		socats.grpc.Cmd.Start()

		d.setFiler(alias, socats)
	}
	return d.filers[alias], nil
}
func (d *Driver) setFiler(alias string, filer *Filer) {
	d.Lock()
	defer d.Unlock()
	d.filers[alias] = filer
}
