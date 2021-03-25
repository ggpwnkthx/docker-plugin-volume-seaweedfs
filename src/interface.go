package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
)

type Volume struct {
	Name, Mountpoint string
	Options          map[string]string
	Filer            struct {
		hostname string
		port     int
	}
	Port      int
	Processes map[string]*exec.Cmd
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	port, err := freeport.GetFreePort()
	if err != nil {
		return err
	}

	v := &Volume{
		Options:    r.Options,
		Name:       r.Name,
		Mountpoint: filepath.Join(d.propagatedMount, r.Name), // "/path/under/PropogatedMount"
		Port:       port,
	}
	v.Processes["socat"] = exec.Command("socat", "tcp-l:localhost:"+strconv.Itoa(v.Port)+",fork", "unix:/run/docker/plugins/seaweedfs/"+v.Name)
	v.Processes["socat"].Start()
	delete(r.Options, "filer")

	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=localhost:" + strconv.Itoa(v.Port),
		"-volumeServerAccess=filerProxy",
	}
	for oKey, oValue := range r.Options {
		if oValue != "" {
			mOptions = append(mOptions, "-"+oKey+"="+oValue)
		} else {
			mOptions = append(mOptions, "-"+oKey)
		}
	}
	v.Processes["weed"] = exec.Command("/usr/bin/weed", mOptions...)
	v.Processes["weed"].Start()
	d.volumes[r.Name] = v

	return nil
}

func (d *Driver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
		})
	}
	return volumes
}

func (d *Driver) mountVolume(v *Volume) error {
	return nil
}

func (d *Driver) removeVolume(v *Volume) error {
	err := os.RemoveAll(d.volumes[v.Name].Mountpoint)
	if err != nil {
		return err
	}
	delete(d.volumes, v.Name)
	return nil
}

func (d *Driver) unmountVolume(v *Volume) error {
	return nil
}
