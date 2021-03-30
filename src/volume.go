package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
)

type Volume struct {
	Driver           *Driver
	Mountpoint, Name string
	Options          map[string]string
	weed             *exec.Cmd
}

func (v *Volume) Create(d *Driver, r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	v.Driver = d
	v.Name = r.Name
	v.Options = r.Options

	err := v.Update()
	if err != nil {
		return err
	}

	return nil
}

func (v *Volume) Update() error {
	filer := strings.Split(v.Options["filer"], ":")[0]
	if filer == "" {
		return errors.New("filer is nil")
	}
	f, err := v.Driver.getFiler(strings.Split(v.Options["filer"], ":")[0])
	if err != nil {
		return err
	}
	v.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, v.Name)
	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=localhost:" + strconv.Itoa(f.http.Port),
		"-volumeServerAccess=filerProxy",
	}
	for oKey, oValue := range v.Options {
		if oKey != "filer" {
			if oValue != "" {
				mOptions = append(mOptions, "-"+oKey+"="+oValue)
			} else {
				mOptions = append(mOptions, "-"+oKey)
			}
		}
	}
	v.weed = exec.Command("/usr/bin/weed", mOptions...)
	v.weed.Stderr = v.Driver.Stderr
	v.weed.Stdout = v.Driver.Stdout
	v.weed.Start()

	if err != nil {
		return err
	}
	return nil
}
func (v *Volume) Remove() error {
	if _, err := os.Stat(v.Mountpoint); !os.IsNotExist(err) {
		err := exec.Command("umount", v.Mountpoint).Run()
		if err != nil {
			return err
		}
		err = os.RemoveAll(v.Mountpoint)
		if err != nil {
			return err
		}
	}
	v.Mountpoint = ""
	return nil
}

func (v *Volume) Mount() error {
	return nil
}

func (v *Volume) Unmount() error {
	return nil
}

func (v *Volume) getStatus() map[string]interface{} {
	status := make(map[string]interface{})
	status["weed"] = v.weed
	return status
}
