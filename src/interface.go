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
	socat            *exec.Cmd
	Sock             string
	weed             *exec.Cmd
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
	sOptions := []string{
		"tcp-l:127.0.0.1:" + strconv.Itoa(v.Port) + ",fork",
		"unix:" + v.Sock,
	}
	v.socat = exec.Command("/usr/bin/socat", sOptions...)
	go func() {
		socatout, _ := os.Create("/var/run/docker/plugins/seaweedfs/" + filer[0] + "/filer.socat.out")
		defer socatout.Close()
		v.socat.Stdout = socatout
		socaterr, _ := os.Create("/var/run/docker/plugins/seaweedfs/" + filer[0] + "/filer.socat.err")
		defer socaterr.Close()
		v.socat.Stdout = socaterr
		v.socat.Run()
	}()

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
	v.weed = exec.Command("/usr/bin/weed", mOptions...)
	go func() {
		weedout, _ := os.Create("/var/run/docker/plugins/seaweedfs/" + filer[0] + "/" + v.Name + ".out")
		defer weedout.Close()
		v.weed.Stdout = weedout
		weederr, _ := os.Create("/var/run/docker/plugins/seaweedfs/" + filer[0] + "/" + v.Name + ".err")
		defer weederr.Close()
		v.weed.Stderr = weederr
		v.socat.Run()
	}()
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
