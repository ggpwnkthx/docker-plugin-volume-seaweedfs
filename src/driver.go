package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

type Driver struct {
	sync.RWMutex
	filers      map[string]*Filer
	socketMount string
	Stderr      *os.File
	Stdout      *os.File
	volumes     map[string]*Volume
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

func loadDriver() *Driver {
	d := &Driver{
		filers:      map[string]*Filer{},
		socketMount: "/var/lib/docker/plugins/seaweedfs/",
		Stdout:      os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout"),
		Stderr:      os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr"),
		volumes:     map[string]*Volume{},
	}
	go d.manage()
	return d
}
func (d *Driver) save() {
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
		logrus.WithField("savePath", savePath).Error(err)
		return
	}
	if err := ioutil.WriteFile(savePath, data, 0644); err != nil {
		logrus.WithField("savestate", savePath).Error(err)
	}
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

func (d *Driver) manage() {
	for {
		syncState := false
		if _, err := os.Stat(savePath); err == nil {
			data, err := ioutil.ReadFile(savePath)
			if err != nil {
				logrus.WithField("loadDriver", savePath).Error(err)
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
