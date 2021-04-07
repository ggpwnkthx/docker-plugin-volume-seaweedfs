package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/fsnotify/fsnotify"
)

type Driver struct {
	sync.RWMutex
	Filers  map[string]*Filer
	Volumes map[string]*Volume
	Watcher struct {
		Notifier *fsnotify.Watcher
		List     map[string]map[string]bool
	}
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
	d.Watcher = struct {
		Notifier *fsnotify.Watcher
		List     map[string]map[string]bool
	}{
		Notifier: watcher,
		List:     map[string]map[string]bool{},
	}
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
	defer d.Watcher.Notifier.Close()
	d.Watcher.Notifier.Add(seaweedfsSockets)
	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-d.Watcher.Notifier.Events:
				if !ok {
					return
				}
				logerr("event:", event.String())
				if event.Op&fsnotify.Create == fsnotify.Create {
					f, _ := os.Open(event.Name)
					fi, _ := f.Stat()
					if fi.IsDir() {
						logerr("wataching additional dir", fi.Name())
						d.Watcher.Notifier.Add(event.Name)
					} else {
						logerr("found new file", fi.Name())
						dir := filepath.Dir(event.Name)
						switch fi.Name() {
						case "http.sock", "grpc.sock":
							d.Watcher.List[dir][fi.Name()] = true
							if d.Watcher.List[dir]["http.sock"] && d.Watcher.List[dir]["grpc.sock"] {
								delete(d.Watcher.List, dir)
								logerr("requirements met, loading filer", dir)
								d.load()
							}
						default:
							d.Watcher.Notifier.Remove(event.Name)
							delete(d.Watcher.List, dir)
							logerr("no longer wataching dir", dir)
						}
					}
				}
			case err, ok := <-d.Watcher.Notifier.Errors:
				if !ok {
					return
				}
				logerr("error:", err.Error())
			}
		}
	}()

	<-done
}
