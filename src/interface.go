package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
	"github.com/sirupsen/logrus"
)

type Driver struct {
	sync.RWMutex
	filers      map[string]*Filer
	socketMount string
	Stderr      *os.File
	Stdout      *os.File
	volumes     map[string]*Volume
}

type Socat struct {
	Cmd  *exec.Cmd
	Port int
	Sock string
}

type Filer struct {
	http *Socat
	grpc *Socat
}

type Volume struct {
	Mountpoint, Name string
	Options          map[string]string
	weed             *exec.Cmd
}

func loadDriver() *Driver {
	d := &Driver{
		socketMount: "/var/lib/docker/plugins/seaweedfs/",
		Stdout:      os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout"),
		Stderr:      os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr"),
		volumes:     map[string]*Volume{},
	}
	if _, err := os.Stat(savePath + ".volumes"); err == nil {
		data, err := ioutil.ReadFile(savePath + ".volumes")
		if err != nil {
			logrus.WithField("loadDriver", savePath).Error(err)
		}
		json.Unmarshal(data, &d.volumes)
		for _, v := range d.volumes {
			d.updateVolume(v)
		}

	} else {
		d.filers = map[string]*Filer{}
	}
	return d
}
func (d *Driver) save() {
	var volumes []Volume
	for _, vValue := range d.volumes {
		v := Volume{
			Name:       vValue.Name,
			Mountpoint: vValue.Mountpoint,
			Options:    vValue.Options,
		}
		volumes = append(volumes, v)
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		logrus.WithField("savePath", savePath).Error(err)
		return
	}
	if err := ioutil.WriteFile(savePath+".volumes", data, 0644); err != nil {
		logrus.WithField("savestate", savePath).Error(err)
	}
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	v := &Volume{
		Mountpoint: filepath.Join(volume.DefaultDockerRootDirectory, r.Name),
		Name:       r.Name,
		Options:    r.Options,
	}
	d.updateVolume(v)
	d.save()

	return nil
}

func (d *Driver) updateVolume(v *Volume) {
	f, err := d.getFiler(strings.Split(v.Options["filer"], ":")[0])
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
	v.weed.Stderr = d.Stderr
	v.weed.Stdout = d.Stdout
	v.weed.Start()

	d.volumes[v.Name] = v
}

func (d *Driver) getVolumeStatus(v *Volume) map[string]interface{} {
	status := make(map[string]interface{})
	status["weed"] = v.weed
	return status
}

func (d *Driver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, v := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Status:     d.getVolumeStatus(v),
		})
	}
	return volumes
}

func (d *Driver) mountVolume(v *Volume) error {
	return nil
}

func (d *Driver) removeVolume(v *Volume) error {
	if _, err := os.Stat(v.Mountpoint); !os.IsNotExist(err) {
		err := exec.Command("umount", v.Mountpoint).Run()
		if err != nil {
			return err
		}
		err = os.RemoveAll(d.volumes[v.Name].Mountpoint)
		if err != nil {
			return err
		}
	}
	delete(d.volumes, v.Name)
	return nil
}

func (d *Driver) unmountVolume(v *Volume) error {
	return nil
}

func (d *Driver) getFiler(alias string) (*Filer, error) {
	_, ok := d.filers[alias]
	if !ok {
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, alias), os.ModeDir)
		port, err := freeport.GetFreePort()
		if err != nil {
			return &Filer{}, errors.New("freeport: " + err.Error())
		}

		socats := &Filer{
			http: &Socat{
				Port: port,
				Sock: filepath.Join(d.socketMount, alias, "http.sock"),
			},
			grpc: &Socat{
				Port: port + 10000,
				Sock: filepath.Join(d.socketMount, alias, "grpc.sock"),
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
		socats.http.Cmd.Stderr = d.Stderr
		socats.http.Cmd.Stdout = d.Stdout
		socats.http.Cmd.Start()

		grpcOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(socats.grpc.Port) + ",fork",
			"unix:" + socats.grpc.Sock,
		}
		socats.grpc.Cmd = exec.Command("/usr/bin/socat", grpcOptions...)
		socats.grpc.Cmd.Stderr = d.Stderr
		socats.grpc.Cmd.Stdout = d.Stdout
		socats.grpc.Cmd.Start()

		d.filers[alias] = socats
	}
	return d.filers[alias], nil
}

func (d *Driver) manage() {

}
