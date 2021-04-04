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
	Filer            *Filer
	Mountpoint, Name string
	Options          map[string]string
	weed             *exec.Cmd
}

func (v *Volume) Create(r *volume.CreateRequest, driver *Driver) error {
	logerr("creating mount " + v.Name + " from filer " + v.Options["filer"])

	if driver.Volumes[r.Name] != nil {
		return errors.New("volume already exists")
	}
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}

	v.Name = r.Name
	v.Options = r.Options
	v.Options["filer"] = strings.Split(r.Options["filer"], ":")[0]
	v.Driver = driver
	v.Filer = driver.Filers[v.Options["filer"]]

	v.Driver.Volumes[v.Name] = v
	//v.Filer.Volumes[v.Name] = r
	v.Filer.saveRunning()
	return nil
}

func (v *Volume) Remove() error {
	if _, err := os.Stat(v.Mountpoint); !os.IsNotExist(err) {
		err := exec.Command("umount", v.Mountpoint).Run()
		if err != nil {
			return err
		}
		v.weed.Wait()
		err = os.RemoveAll(v.Mountpoint)
		if err != nil {
			return err
		}
	}
	delete(v.Driver.Volumes, v.Name)
	//delete(v.Filer.Volumes, v.Name)
	v.Filer.saveRunning()
	return nil
}

func (v *Volume) Mount() error {
	logerr("mounting " + v.Name)
	if v.weed == nil {
		v.Mountpoint = filepath.Join(volume.DefaultDockerRootDirectory, v.Name)
		mOptions := []string{
			"mount",
			//"-allowOthers",
			"-dir=" + v.Mountpoint,
			//"-dirAutoCreate",
			"-filer=localhost:" + strconv.Itoa(v.Filer.http.Port),
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
		os.MkdirAll(v.Mountpoint, os.ModePerm)
		v.weed = exec.Command("/usr/bin/weed", mOptions...)
		v.weed.Stderr = Stderr
		v.weed.Stdout = Stdout
		v.weed.Start()
	}
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
