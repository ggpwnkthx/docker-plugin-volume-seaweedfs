package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/fsnotify/fsnotify"
	client_native "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	runtime_api "github.com/haproxytech/client-native/v2/runtime"
)

type Driver struct {
	sync.RWMutex
	Filers  map[string]*Filer
	Volumes map[string]*Volume
	HAProxy *client_native.HAProxyClient
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

	// Initialize HAProxy native client
	hapcc := &configuration.Client{}
	hapcp := configuration.ClientParams{
		ConfigurationFile:      "/etc/haproxy/haproxy.cfg",
		Haproxy:                "/usr/sbin/haproxy",
		UseValidation:          true,
		PersistentTransactions: true,
		TransactionDir:         "/etc/haproxy/transactions",
	}
	err := hapcc.Init(hapcp)
	if err != nil {
		logerr("Error setting up default configuration client, exiting...", err.Error())
	}
	haprtc := &runtime_api.Client{}
	version, globalConf, err := hapcc.GetGlobalConfiguration("")
	logerr("GetGlobalConfiguration:", "version", strconv.FormatInt(version, 10))
	if err == nil {
		socketList := map[int]string{}
		runtimeAPIs := globalConf.RuntimeAPIs

		if len(runtimeAPIs) != 0 {
			for i, r := range runtimeAPIs {
				socketList[i] = *r.Address
			}
			if err := haprtc.InitWithSockets(socketList); err != nil {
				logerr("Error setting up runtime client, not using one")
				return nil
			}
		} else {
			logerr("Runtime API not configured, not using it")
			haprtc = nil
		}
	} else {
		logerr("Cannot read runtime API configuration, not using it")
		haprtc = nil
	}

	d.HAProxy = &client_native.HAProxyClient{}
	d.HAProxy.Init(hapcc, haprtc)

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
	for alias, f := range d.Filers {
		if !Contains(filers, alias) {
			d.removeFiler(f)
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

func (d *Driver) isFiler(alias string) bool {
	d.RLock()
	defer d.RUnlock()
	if _, found := d.Filers[alias]; found {
		return true
	}
	return false
}
func (d *Driver) addFiler(f *Filer) {
	d.Lock()
	defer d.Unlock()
	d.Filers[f.alias] = f
}
func (d *Driver) removeFiler(f *Filer) {
	d.Lock()
	defer d.Unlock()
	delete(d.Filers, f.alias)
}

func (d *Driver) addVolume(v *Volume) {
	d.Lock()
	defer d.Unlock()
	d.Volumes[v.Name] = v
}
func (d *Driver) deleteVolume(v *Volume) {
	d.Lock()
	defer d.Unlock()
	delete(d.Volumes, v.Name)
}
func (d *Driver) removeVolume(v *Volume) error {
	d.deleteVolume(v)
	return v.Filer.saveRunning()
}

func (d *Driver) watcher() {
	d.addDirWatch(seaweedfsSockets)
	defer d.Watcher.Notifier.Close()
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
						d.addDirWatch(event.Name)
						if isFiler(fi.Name()) {
							d.removeDirWatch(fi.Name())
							d.load()
						}
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
	delete(d.Watcher.List, dir)
}
func (d *Driver) addFoundSocket(alias string, socket string) {
	d.Lock()
	defer d.Unlock()
	d.Watcher.List[alias][socket] = true
}
