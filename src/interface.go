package main

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

type Volume struct {
	Name, Mountpoint string
	Options          map[string]string
	CMD              *exec.Cmd
}

func (d *Driver) createVolume(v *Volume) error {
	_, ok := v.Options["filer"]
	if !ok {
		return errors.New("No filer address:port specified. No connection can be made.")
	}
	var client = http.Client{
		Timeout: time.Duration(3) * time.Second,
	}
	url := "http://" + v.Options["filer"]
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return err
	}
	_, err = client.Do(req)
	if err != nil {
		return err
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
	d.volumes[v.Name] = &Volume{
		Options:    v.Options,
		Name:       v.Name,
		Mountpoint: v.Mountpoint,
		CMD:        exec.Command("/usr/bin/weed", mOptions...),
	}

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

/*
func manage(d *Driver, v *Volume) {
	if d.volumes[v.Name] != nil {
		d.sync.RLock()
		outbuf := make([]byte, 1024)
		outn, _ := d.volumes[v.Name].Exec.stdout.Read(outbuf)
		errbuf := make([]byte, 1024)
		errn, _ := d.volumes[v.Name].Exec.stderr.Read(errbuf)
		d.sync.RUnlock()
		if outn > 0 {
			d.sync.Lock()
			d.volumes[v.Name].Exec.logs.out += string(outbuf[0:outn])
			d.sync.Unlock()
		}
		if errn > 0 {
			d.sync.Lock()
			d.volumes[v.Name].Exec.logs.err += string(errbuf[0:errn])
			d.sync.Unlock()
		}
	}
}
*/
