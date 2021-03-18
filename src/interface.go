package main

import (
	"os"
	"path/filepath"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Options          []string
	Name, Mountpoint string
	PID, Connections int
}

func (d *volumeDriver) getVolumeByName(name string) (*dockerVolume, error) {
	return d.volumes[name], nil
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
	}
	return nil
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	v.Mountpoint = filepath.Join(d.propagatedMount, v.Name)
	d.volumes[v.Name] = v
	return nil
}
