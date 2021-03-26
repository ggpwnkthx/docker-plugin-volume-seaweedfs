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

type Socat struct {
	Cmd      *exec.Cmd
	Port     int
	SockPath string
}

var socats map[string]Socat

type Volume struct {
	Filer            []string
	Mountpoint, Name string
	Options          map[string]string
	socat            Socat
	weed             *exec.Cmd
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	filer := strings.Split(r.Options["filer"], ":")
	delete(r.Options, "filer")

	_, ok = socats[filer[0]]
	if !ok {
		port, err := freeport.GetFreePort()
		if err != nil {
			return errors.New("freeport: " + err.Error())
		}
		socats[filer[0]] = Socat{
			Port:     port,
			SockPath: d.socketMount + filer[0],
		}
	}
	s := socats[filer[0]]
	if s.Cmd == nil {
		sOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:127.0.0.1:" + strconv.Itoa(socats[filer[0]].Port) + ",fork",
			"unix:" + socats[filer[0]].SockPath + "/filer.sock",
		}
		s.Cmd = exec.Command("/usr/bin/socat", sOptions...)
		s.Cmd.Start()
	}

	v := &Volume{
		Filer:      filer,
		Mountpoint: filepath.Join(d.propagatedMount, r.Name), // "/path/under/PropogatedMount"
		Options:    r.Options,
		Name:       r.Name,
		socat:      s,
	}
	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=127.0.0.1:" + strconv.Itoa(v.socat.Port),
		"-volumeServerAccess=filerProxy",
	}
	for oKey, oValue := range r.Options {
		if oValue != "" {
			mOptions = append(mOptions, "-"+oKey+"="+oValue)
		} else {
			mOptions = append(mOptions, "-"+oKey)
		}
	}
	v.weed = exec.Command("/usr/bin/weed", mOptions...)
	v.weed.Start()

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
