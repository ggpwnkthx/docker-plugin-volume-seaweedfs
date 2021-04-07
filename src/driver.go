package main

import (
	"errors"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/fsnotify/fsnotify"
)

type Driver struct {
	sync.RWMutex
	Filers  map[string]*Filer
	Volumes map[string]*Volume
	Watcher *fsnotify.Watcher
}

func (d *Driver) init() error {
	if d.Filers == nil {
		d.Filers = map[string]*Filer{}
	}
	if d.Volumes == nil {
		d.Volumes = map[string]*Volume{}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logerr(err.Error())
	}
	d.Watcher = watcher
	go d.watcher()
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
	v.Filer.saveRunning()
	return nil
}
func (d *Driver) getVolume(name string) (*Volume, error) {
	d.load()
	if v, found := d.Volumes[name]; found {
		return v, nil
	}
	return nil, errors.New("volume " + name + " not found")
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

func (d *Driver) watcher() {
	defer d.Watcher.Close()
	d.Watcher.Add(seaweedfsSockets)
	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-d.Watcher.Events:
				if !ok {
					return
				}
				logerr("event:", event.String())
				if event.Op&fsnotify.Write == fsnotify.Write {
					logerr("modified file:", event.Name)
				}
			case err, ok := <-d.Watcher.Errors:
				if !ok {
					return
				}
				logerr("error:", err.Error())
			}
		}
	}()

	<-done
}
