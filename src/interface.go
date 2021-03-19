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

func (d *volumeDriver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, mount := range d.volumes {
		var v volume.Volume
		v.Name = mount.Name
		v.Mountpoint = mount.Mountpoint
		volumes = append(volumes, &v)
	}
	return volumes
}

func (d *volumeDriver) mountVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	defer d.volumes[v.Name].sync.Unlock()
	d.volumes[v.Name].Connections++
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	defer d.volumes[v.Name].sync.Unlock()
	if d.volumes[v.Name].Connections == 0 {
		err := d.volumes[v.Name].CMD.Process.Kill()
		if err != nil {
			return err
		}
		err = os.RemoveAll(d.volumes[v.Name].Mountpoint)
		if err != nil {
			return err
		}
		delete(d.volumes, v.Name)
		return nil
	} else {
		return errors.New("There are still " + strconv.Itoa(v.Connections) + " active connections.")
	}
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	d.volumes[v.Name].sync.Lock()
	defer d.volumes[v.Name].sync.Unlock()
	d.volumes[v.Name].Connections--
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	if _, found := d.volumes[v.Name]; found {
		d.volumes[v.Name].sync.Lock()
		defer d.volumes[v.Name].sync.Unlock()
		d.volumes[v.Name] = v
	} else {
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
		d.volumes[v.Name].CMD = exec.Command("/usr/bin/weed", mOptions...)
		d.volumes[v.Name].Tries = 0
		d.volumes[v.Name].sync = &sync.Mutex{}
	}
	return nil
}

func (d *volumeDriver) manager() {
	for _, v := range d.volumes {
		if v.CMD.ProcessState.Exited() {
			if v.Tries < 3 {
				v.sync.Unlock()
				v.CMD.Start()
				v.sync.Lock()
			} else {
				d.removeVolume(v)
			}
		}
	}
}
