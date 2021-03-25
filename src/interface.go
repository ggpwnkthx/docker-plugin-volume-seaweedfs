package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
)

type Volume struct {
	Filer            []string
	Mountpoint, Name string
	Options          map[string]string
	Port             int
	Processes        map[string]*exec.Cmd
	Sock             string
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	filer := strings.Split(r.Options["filer"], ":")
	delete(r.Options, "filer")

	port, err := freeport.GetFreePort()
	if err != nil {
		return errors.New("freeport: " + err.Error())
	}

	v := &Volume{
		Filer:      filer,
		Mountpoint: filepath.Join(d.propagatedMount, r.Name), // "/path/under/PropogatedMount"
		Options:    r.Options,
		Name:       r.Name,
		Port:       port,
		Sock:       "/var/run/docker/plugins/seaweedfs/" + filer[0] + "/filer.sock",
	}
	if err == nil {
		return errors.New("volume: declared")
	}
	v.Processes["socat"] = exec.Command("socat", "tcp-l:127.0.0.1:"+strconv.Itoa(v.Port)+",fork", "unix:"+v.Sock)
	err = v.Processes["socat"].Start()
	if err != nil {
		return errors.New("socat: " + err.Error())
	}

	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=127.0.0.1:" + strconv.Itoa(v.Port),
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
	err = v.Processes["weed"].Start()
	if err != nil {
		return errors.New("weed: " + err.Error())
	}
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
