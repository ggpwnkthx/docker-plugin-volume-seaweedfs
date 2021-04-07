package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
)

type Volume struct {
	volume.Volume
	Driver  *Driver
	Filer   *Filer
	Options map[string]string
}

func (v *Volume) Create(r *volume.CreateRequest, driver *Driver) error {
	if driver.Volumes[r.Name] != nil {
		return errors.New("volume already exists")
	}
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	v.Name = r.Name
	v.Options = map[string]string{}
	v.Options = r.Options
	v.Options["filer"] = strings.Split(r.Options["filer"], ":")[0]
	logerr("creating mount " + v.Name + " from filer " + v.Options["filer"])
	v.Driver = driver
	if _, found := driver.Filers[v.Options["filer"]]; !found {
		f := new(Filer)
		err := f.load(v.Options["filer"], v.Driver)
		if err != nil {
			return err
		}
	}
	v.Filer = driver.Filers[v.Options["filer"]]
	if _, found := v.Options["filer.path"]; found {
		v.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, v.Filer.alias, v.Options["filer.path"])
	} else {
		v.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, v.Filer.alias)
	}

	v.Driver.Volumes[v.Name] = v
	return nil
}

func (v *Volume) Remove() error {
	logerr("removing mount " + v.Name)
	delete(v.Driver.Volumes, v.Name)
	v.Filer.saveRunning()
	return nil
}

func (v *Volume) Mount() error {
	logerr("mounting " + v.Name)
	return nil
}

func (v *Volume) Unmount() error {
	logerr("unmounting mount " + v.Name)
	return nil
}

func (v *Volume) getStatus() map[string]interface{} {
	logerr("getting status of mount " + v.Name)
	status := make(map[string]interface{})
	status["options"] = v.Options
	return status
}
