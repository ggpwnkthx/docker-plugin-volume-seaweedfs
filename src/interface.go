package main

import (
	"os"
	"os/exec"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Options          []string
	Name, Mountpoint string
	PID, Connections int
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
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	if v.Connections == 0 {
		cmd, _ := os.FindProcess(v.PID)
		err := cmd.Kill()
		if err != nil {
			return err
		}
		delete(d.volumes, v.Name)
	}
	return nil
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	if _, found := d.volumes[v.Name]; found {
		d.volumes[v.Name] = v
	} else {
		if _, err := os.Stat(v.Mountpoint); err != nil {
			if os.IsNotExist(err) {
				os.Mkdir(v.Mountpoint, 0755)
			}
		}
		var args []string
		args = append(args, "mount")
		args = append(args, "-dir="+v.Mountpoint)
		args = append(args, "-dirAutoCreate")
		args = append(args, "-volumeServerAccess=filerProxy")
		for _, option := range v.Options {
			args = append(args, option)
		}
		cmd := exec.Command("/usr/bin/weed", args...)
		err := cmd.Run()
		if err != nil {
			return err
		}
		v.PID = cmd.Process.Pid
		d.volumes[v.Name] = v
	}
	return nil
}
