package main

import (
	"errors"
	"os"
	"os/exec"

	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Name, Mountpoint string
	Options          map[string]string
	CMD              *exec.Cmd
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
		Options:    v.Options,
		Name:       v.Name,
		Mountpoint: v.Mountpoint,
		CMD:        exec.Command("/usr/bin/weed", mOptions...),
	}
	//d.volumes[v.Name].CMD.Start()

	return nil
}

func (d *volumeDriver) getVolumeStatus(v *dockerVolume) map[string]interface{} {
	var status map[string]interface{}
	status["weed"] = d.volumes[v.Name].CMD
	return status
}

func (d *volumeDriver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
		})
	}
	return volumes
}

func (d *volumeDriver) mountVolume(v *dockerVolume) error {
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	err := os.RemoveAll(d.volumes[v.Name].Mountpoint)
	if err != nil {
		return err
	}
	delete(d.volumes, v.Name)
	return nil
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	return nil
}

/*
func manage(d *volumeDriver, v *dockerVolume) {
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
