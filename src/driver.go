package main

import (
	"errors"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
)

type Driver struct {
	sync.RWMutex
	volumes map[string]*Volume
}

func (d *Driver) init() {
	d.Lock()
	defer d.Unlock()
	d.volumes = make(map[string]*Volume)
}

func (d *Driver) load() error {
	d.init()
	filers, err := availableFilers()
	if err != nil {
		return err
	}
	for _, f := range filers {
		filer, err := getFiler(f)
		if err != nil {
			return err
		}
		requests, err := filer.listVolumes()
		if err != nil {
			return err
		}
		for _, r := range *requests {
			r.Options["filer"] = f
			d.addVolume(&r)
		}
	}
	return nil
}
func (d *Driver) save() error {
	filers := map[string][]volume.CreateRequest{}
	volumes := d.activeVolumes()
	for _, v := range volumes {
		vol := volume.CreateRequest{
			Name:    v.Name,
			Options: v.Options,
		}
		delete(vol.Options, "filer")
		filers[v.Options["filer"]] = append(filers[v.Options["filer"]], vol)
	}
	for a, v := range filers {
		filer, err := getFiler(a)
		if err != nil {
			return err
		}
		err = filer.saveVolumes(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) activeVolumes() map[string]*Volume {
	d.RLock()
	defer d.RUnlock()
	return d.volumes
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
func (d *Driver) addVolume(r *volume.CreateRequest) error {
	v := new(Volume)
	err := v.Create(r)
	if err != nil {
		return err
	}
	err = d.updateVolume(v)
	if err != nil {
		return err
	}
	return nil
}
func (d *Driver) createVolume(r *volume.CreateRequest) error {
	d.addVolume(r)
	return d.save()
}
func (d *Driver) listVolumes() []*volume.Volume {
	d.load()
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
