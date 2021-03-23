package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	CreatedAt          string
	Options            map[string]string
	Name, Mountpoint   string
	Status             map[string]interface{}
	Connections, Tries int
	CMD                *exec.Cmd
	stdout             io.ReadCloser
	stderr             io.ReadCloser
	logs               struct {
		out string
		err string
	}
}

func (d *volumeDriver) createVolume(v *dockerVolume) error {
	d.sync.Lock()
	defer d.sync.Unlock()

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
		"mount",
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
	d.volumes[v.Name] = &dockerVolume{
		Options:     v.Options,
		Name:        v.Name,
		Mountpoint:  v.Mountpoint,
		Status:      make(map[string]interface{}),
		Connections: 0,
		Tries:       0,
		CMD:         exec.Command("/usr/bin/weed", mOptions...),
		logs: struct {
			out string
			err string
		}{},
	}
	d.volumes[v.Name].stdout, _ = d.volumes[v.Name].CMD.StdoutPipe()
	d.volumes[v.Name].stderr, _ = d.volumes[v.Name].CMD.StderrPipe()
	if err := d.volumes[v.Name].CMD.Start(); err != nil {
		return err
	}
	go func(d *volumeDriver, v *dockerVolume) {
		buf := make([]byte, 1024)
		for {
			d.sync.Lock()
			n, err := d.volumes[v.Name].stdout.Read(buf)
			if err != nil {
				d.volumes[v.Name].logs.err += err.Error()
			}
			d.volumes[v.Name].logs.out += string(buf[0:n])
			n, err = d.volumes[v.Name].stderr.Read(buf)
			if err != nil {
				d.volumes[v.Name].logs.err += err.Error()
			}
			d.volumes[v.Name].logs.err += string(buf[0:n])
			d.sync.Unlock()
			time.Sleep(2 * time.Second)
		}
	}(d, v)

	return nil
}

func (d *volumeDriver) updateVolumeStatus(v *dockerVolume) {
	d.sync.Lock()
	defer d.sync.Unlock()
	v.Status["weed"] = v.CMD
	v.Status["logs"] = v.logs
}

func (d *volumeDriver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		d.updateVolumeStatus(v)
		volumes = append(volumes, &volume.Volume{
			CreatedAt:  v.CreatedAt,
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Status:     v.Status,
		})
	}
	return volumes
}

func (d *volumeDriver) mountVolume(v *dockerVolume) error {
	d.sync.Lock()
	defer d.sync.Unlock()
	d.volumes[v.Name].Connections++
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	d.sync.Lock()
	defer d.sync.Unlock()
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
	d.sync.Lock()
	defer d.sync.Unlock()
	d.volumes[v.Name].Connections--
	return nil
}

func copyLogs(r io.Reader, logfn func(args ...interface{})) {
	buf := make([]byte, 80)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			logfn(buf[0:n])
		}
		if err != nil {
			break
		}
	}
}
