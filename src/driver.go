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
	d.Lock()
	defer d.Unlock()
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
	d.RLock()
	defer d.RUnlock()
	if v, found := d.Volumes[name]; found {
		return v, nil
	}
	return nil, errors.New("volume " + name + " not found")
}
func (d *Driver) listVolumes() []*volume.Volume {
	d.load()
	d.RLock()
	defer d.RUnlock()
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
					d.Lock()
					if fi.IsDir() {
						d.addDirWatch(event.Name)
					} else {
						logerr("found new file", fi.Name())
						dir := filepath.Dir(event.Name)
						switch fi.Name() {
						case "http.sock", "grpc.sock":
							d.addFoundSocket(dir, fi.Name())
							if isFiler(dir) {
								d.removeDirWatch(dir)
								d.load()
							}
						default:
							d.removeDirWatch(dir)
						}
					}
					d.Unlock()
				}
			case err, ok := <-d.Watcher.Notifier.Errors:
				logerr("error:", err.Error())
				if !ok {
					return
				}
			}
		}
	}()

	<-done
}
func (d *Driver) addDirWatch(dir string) {
	d.Lock()
	defer d.Unlock()
	logerr("wataching additional dir", dir)
	err := d.Watcher.Notifier.Add(dir)
	if err != nil {
		logerr(err.Error())
	}
}
func (d *Driver) removeDirWatch(dir string) {
	d.Lock()
	defer d.Unlock()
	logerr("no longer wataching dir", dir)
	err := d.Watcher.Notifier.Remove(dir)
	if err != nil {
		logerr(err.Error())
	}
	if _, found := d.Watcher.List[dir]; found {
		delete(d.Watcher.List, dir)
	}
}
func (d *Driver) addFoundSocket(alias string, socket string) {
	d.Lock()
	defer d.Unlock()
	d.Watcher.List[alias][socket] = true
}
