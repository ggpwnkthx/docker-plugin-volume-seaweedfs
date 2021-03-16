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

var mountedVolumes []dockerVolume

func getVolumeByName(name string) (*dockerVolume, error) {
	for _, mount := range mountedVolumes {
		if mount.Name == name {
			return &mount, nil
		}
	}
	return nil, nil
}

func listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, mount := range mountedVolumes {
		var v volume.Volume
		v.Name = mount.Name
		v.Mountpoint = mount.Mountpoint
		volumes = append(volumes, &v)
	}
	return volumes
}

func mountVolume(v *dockerVolume) error {
	return nil
}

func removeVolume(v *dockerVolume) error {
	if v.Connections == 0 {
		cmd, _ := os.FindProcess(v.PID)
		err := cmd.Kill()
		if err != nil {
			return err
		}
	}
	return nil
}

func unmountVolume(v *dockerVolume) error {
	return nil
}

func updateVolume(v *dockerVolume) error {
	id := -1
	for index, mount := range mountedVolumes {
		if mount.Name == v.Name {
			id = index
		}
	}
	if id <= 0 {
		mountedVolumes[id] = *v
	} else {
		v.Mountpoint = filepath.Join(propagatedMount, v.Name)
		if _, err := os.Stat(v.Mountpoint); err != nil {
			if os.IsNotExist(err) {
				os.Mkdir(v.Mountpoint, 0755)
			}
		}
		/*
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
			v.PID = cmd.Process.Pid
		*/
		mountedVolumes = append(mountedVolumes, *v)
		/*
			if err != nil {
				return err
			}
		*/
	}
	return nil
}
