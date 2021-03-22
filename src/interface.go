package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Options            map[string]string
	Name, Mountpoint   string
	Connections, Tries int
	CMD                *exec.Cmd
	sync               *sync.Mutex
}

func (d *volumeDriver) createVolume(v *dockerVolume) error {
	_, ok := v.Options["filer"]
	if !ok {
		return errors.New("No filer name or address specified. No connection can be made.")
	}
	if _, err := os.Stat(v.Mountpoint); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(v.Mountpoint, 760)
		}
	}
	mOptions := []string{
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-volumeServerAccess=filerProxy",
	}
	for oKey, oValue := range v.Options {
		if oValue != "" {
			mOptions = append(mOptions, "-"+oKey+"="+oValue)
		} else {
			mOptions = append(mOptions, "-"+oKey)
		}
	}
	d.volumes[v.Name] = v
	d.volumes[v.Name].sync = &sync.Mutex{}
	d.volumes[v.Name].Tries = 1
	d.volumes[v.Name].CMD = exec.Command("/usr/bin/weed", mOptions...)
	d.volumes[v.Name].CMD.Start()

	return nil
}

func (d *volumeDriver) getVolume(name string) *volume.Volume {
	d.volumes[name].sync.Lock()
	var v volume.Volume
	v.Name = d.volumes[name].Name
	v.Mountpoint = d.volumes[name].Mountpoint
	v.Status["Args"] = d.volumes[name].CMD.Args
	v.Status["Dir"] = d.volumes[name].CMD.Dir
	v.Status["Env"] = d.volumes[name].CMD.Env
	v.Status["Path"] = d.volumes[name].CMD.Path
	v.Status["String"] = d.volumes[name].CMD.String()
	v.Status["ProcessState"] = d.volumes[name].CMD.ProcessState
	if d.volumes[name].CMD.ProcessState.Exited() {
		v.Status["Stderr"] = d.volumes[name].CMD.Stderr
		v.Status["Stdout"] = d.volumes[name].CMD.Stdout
	}
	d.volumes[name].sync.Unlock()
	return &v
}

func (d *volumeDriver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		volumes = append(volumes, d.getVolume(v.Name))
	}
	return volumes
}

func (d *volumeDriver) mountVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	d.volumes[v.Name].Connections++
	d.volumes[v.Name].sync.Unlock()
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	defer d.volumes[v.Name].sync.Unlock()
	if d.volumes[v.Name].Connections < 1 {
		err := os.RemoveAll(d.volumes[v.Name].Mountpoint)
		if err != nil {
			return err
		}
		delete(d.volumes, v.Name)
		return nil
	}
	return errors.New("There are still " + strconv.Itoa(v.Connections) + " active connections.")
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	defer d.volumes[v.Name].sync.Unlock()
	d.volumes[v.Name].Connections--
	return nil
}

/*
func (d *volumeDriver) manager() {
	for {
		for _, v := range d.volumes {
			v.sync.Lock()
			v.Status["Args"] = v.CMD.Args
			v.Status["Dir"] = v.CMD.Dir
			v.Status["Env"] = v.CMD.Env
			v.Status["Path"] = v.CMD.Path
			v.Status["String"] = v.CMD.String()
			v.Status["ProcessState"] = v.CMD.ProcessState
			if v.CMD.ProcessState.Exited() {
				_, stderr := v.Status["Stderr"]
				if !stderr {
					v.Status["Stderr"] = v.CMD.Stderr
				}
				_, stdout := v.Status["Stdout"]
				if !stdout {
					v.Status["Stdout"] = v.CMD.Stdout
				}
			}
			v.sync.Unlock()
		}
	}
}
*/
