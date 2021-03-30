package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

type Driver struct {
	sync.RWMutex
	savePath string
	volumes  map[string]*Volume
}

func (d *Driver) load(savePath string) error {
	d.Lock()
	d.savePath = savePath
	d.volumes = make(map[string]*Volume)
	d.Unlock()

	go d.manage()

	if _, err := os.Stat(d.savePath); err == nil {
		data, err := ioutil.ReadFile(d.savePath)
		if err != nil {
			return err
		}
		var volumes []volume.CreateRequest
		json.Unmarshal(data, volumes)
		for _, r := range volumes {
			err := d.createVolume(&r)
			if err != nil {
				return err
			}
		}
	} else {
		logerr("no save found")
	}
	return nil
}
func (d *Driver) save() error {
	var volumes []volume.CreateRequest
	d.RLock()
	defer d.RUnlock()
	for _, v := range d.volumes {
		volumes = append(volumes, volume.CreateRequest{
			Name:    v.Name,
			Options: v.Options,
		})
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(d.savePath, data, 0644); err != nil {
		return err
	}
	return nil
}

func (d *Driver) updateVolume(v *Volume) error {
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
	return nil
}
func (d *Driver) createVolume(r *volume.CreateRequest) error {
	v := new(Volume)
	err := v.Create(r)
	if err != nil {
		return err
	}
	err = d.updateVolume(v)
	if err != nil {
		return err
	}
	return d.save()
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

func (d *Driver) removeVolume(v *Volume) error {
	v.Remove()
	err := d.updateVolume(v)
	if err != nil {
		return err
	}
	return d.save()
}

func (d *Driver) manage() {
	for {
		for _, v := range d.volumes {
			if v.weed == nil {
				logerr(v.Name + ": mount not running")
				v.Update()
			}
		}
		time.Sleep(5 * time.Second)
	}
}
