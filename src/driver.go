package main

import (
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
)

type Driver struct {
	sync.RWMutex
	Filers  map[string]*Filer
	Volumes map[string]*Volume
}

func (d *Driver) init() error {
	if d.Filers == nil {
		d.Filers = map[string]*Filer{}
	}
	if d.Volumes == nil {
		d.Volumes = map[string]*Volume{}
	}
	return d.load()
}
func (d *Driver) load() error {
	filers, err := availableFilers()
	if err != nil {
		return err
	}
	for _, alias := range filers {
		filer := new(Filer)
		err := filer.load(alias, d)
		if err != nil {
			return err
		}
	}
	for alias := range d.Filers {
		if !Contains(filers, alias) {
			delete(d.Filers, alias)
		}
	}
	return nil
}

/*
func (d *Driver) save() error {
	for _, filer := range Filers {
		err := filer.saveRunning()
		if err != nil {
			return err
		}
	}
	return nil
}
*/

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	v := new(Volume)
	err := v.Create(r, d)
	if err != nil {
		return err
	}
	return nil
}
func (d *Driver) listVolumes() []*volume.Volume {
	d.load()
	var volumes []*volume.Volume
	for _, v := range d.Volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Status:     v.getStatus(),
		})
	}
	return volumes
}

func (d *Driver) removeVolume(v *Volume) error {
	return v.Remove()
}
