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
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	err := d.unmountVolume(v)
	if err != nil {
		return err
	}
	delete(d.volumes, v.Name)
	return nil
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	if v.Connections == 0 {
		cmd, _ := os.FindProcess(v.PID)
		err := cmd.Kill()
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	d.volumes[v.Name] = v
	if _, err := os.Stat(v.Mountpoint); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(v.Mountpoint, 760)
		}
	}
	return nil
}
