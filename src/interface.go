package main

import (
	"errors"
	"os"
	"os/exec"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Options          map[string]string
	Name, Mountpoint string
	Connections      int
	CMD              *exec.Cmd
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
	v.Connections++
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	if v.Connections == 0 {
		err := d.volumes[v.Name].CMD.Process.Kill()
		if err != nil {
			return err
		}
		err = os.RemoveAll(v.Mountpoint)
		if err != nil {
			return err
		}
		delete(d.volumes, v.Name)
		return nil
	} else {
		return errors.New("Active connections still exist.")
	}
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	v.Connections--
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	if _, found := d.volumes[v.Name]; found {
		d.volumes[v.Name] = v
	} else {
		_, ok := v.Options["filer"]
		if !ok {
			return errors.New("No filer name or address specified. No connection can be made.")
		}
		d.volumes[v.Name] = v
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
		d.volumes[v.Name].CMD = exec.Command("/usr/bin/weed", mOptions...)
		d.volumes[v.Name].CMD.Start()
	}
	return nil
}
