package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"

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

	if _, err := os.Stat(d.savePath); err == nil {
		data, err := ioutil.ReadFile(d.savePath)
		if err != nil {
			return err
		}
		var volumes []volume.CreateRequest
		json.Unmarshal(data, volumes)
		for _, r := range volumes {
			_, err := d.addVolume(&r)
			if err != nil {
				return err
			}
		}
	} else {
		return err
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
	d.save()
	return nil
}
func (d *Driver) addVolume(r *volume.CreateRequest) (*Volume, error) {
	v := new(Volume)
	err := v.Create(r)
	if err != nil {
		return &Volume{}, err
	}
	return v, nil
}
func (d *Driver) createVolume(r *volume.CreateRequest) error {
	v, err := d.addVolume(r)
	if err != nil {
		return err
	}
	return d.updateVolume(v)
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
	return d.updateVolume(v)
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
