package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
)

type Volume struct {
	Driver           *Driver
	Mountpoint, Name string
	Options          map[string]string
	weed             *exec.Cmd
}

func CreateVolume(d *Driver, r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	v := &Volume{
		Driver:     d,
		Mountpoint: filepath.Join(volume.DefaultDockerRootDirectory, r.Name),
		Name:       r.Name,
		Options:    r.Options,
	}
	v.Update()
	v.Driver.save()

	return nil
}

func (v *Volume) Update() {
	filer := strings.Split(v.Options["filer"], ":")[0]
	if filer == "" {
		logrus.WithField("filer", filer).Error(errors.New("filer is nil"))
		return
	}
	f, err := v.getFiler()
	if err != nil {
		logrus.WithField("getFiler", f).Error(err)
		return
	}
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

	v.Driver.Lock()
	defer v.Driver.Unlock()
	v.Driver.volumes[v.Name] = v
}

func (v *Volume) Mount() error {
	return nil
}

func (v *Volume) Unmount() error {
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
	v.Driver.Lock()
	delete(v.Driver.volumes, v.Name)
	v.Driver.Unlock()
	v.Driver.save()
	return nil
}

func (v *Volume) getStatus() map[string]interface{} {
	status := make(map[string]interface{})
	status["weed"] = v.weed
	return status
}

func (v *Volume) getFiler() (*Filer, error) {
	alias := strings.Split(v.Options["filer"], ":")[0]
	v.Driver.RLock()
	_, ok := v.Driver.filers[alias]
	v.Driver.RUnlock()
	if !ok {
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, alias), os.ModeDir)
		port := 0
		for {
			port, err := freeport.GetFreePort()
			if err != nil {
				return &Filer{}, errors.New("freeport: " + err.Error())
			}
			if port < 55535 {
				break
			}
		}

		socats := &Filer{
			http: &Socat{
				Port: port,
				Sock: filepath.Join(v.Driver.socketMount, alias, "http.sock"),
			},
			grpc: &Socat{
				Port: port + 10000,
				Sock: filepath.Join(v.Driver.socketMount, alias, "grpc.sock"),
			},
		}
		if _, err := os.Stat(socats.http.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("http unix socket not found")
		}
		if _, err := os.Stat(socats.grpc.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("grpc unix socket not found")
		}

		httpOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(socats.http.Port) + ",fork",
			"unix:" + socats.http.Sock,
		}
		socats.http.Cmd = exec.Command("/usr/bin/socat", httpOptions...)
		socats.http.Cmd.Stderr = v.Driver.Stderr
		socats.http.Cmd.Stdout = v.Driver.Stdout
		socats.http.Cmd.Start()

		grpcOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(socats.grpc.Port) + ",fork",
			"unix:" + socats.grpc.Sock,
		}
		socats.grpc.Cmd = exec.Command("/usr/bin/socat", grpcOptions...)
		socats.grpc.Cmd.Stderr = v.Driver.Stderr
		socats.grpc.Cmd.Stdout = v.Driver.Stdout
		socats.grpc.Cmd.Start()
		v.Driver.Lock()
		defer v.Driver.Unlock()
		v.Driver.filers[alias] = socats
	}
	return v.Driver.filers[alias], nil
}
