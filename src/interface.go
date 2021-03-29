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
	"time"

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
		filers:      map[string]*Filer{},
		socketMount: "/var/lib/docker/plugins/seaweedfs/",
		Stdout:      os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout"),
		Stderr:      os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr"),
		volumes:     map[string]*Volume{},
	}
	go d.manage()
	return d
}
func (d *Driver) save() {
	var volumes []Volume
	d.RLock()
	defer d.RUnlock()
	for _, v := range d.volumes {
		volumes = append(volumes, Volume{
			Name:    v.Name,
			Options: v.Options,
		})
	}
	data, err := json.Marshal(volumes)
	if err != nil {
		logrus.WithField("savePath", savePath).Error(err)
		return
	}
	if err := ioutil.WriteFile(savePath, data, 0644); err != nil {
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
	filer := strings.Split(v.Options["filer"], ":")[0]
	if filer == "" {
		logrus.WithField("filer", filer).Error(errors.New("filer is nil"))
		return
	}
	f, err := d.getFiler(filer)
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

	d.Lock()
	defer d.Unlock()
	d.volumes[v.Name] = v
}

func (d *Driver) getVolumeStatus(v *Volume) map[string]interface{} {
	status := make(map[string]interface{})
	status["weed"] = v.weed
	return status
}

func (d *Driver) listVolumes() []*volume.Volume {
	d.RLock()
	defer d.RUnlock()
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
		err = os.RemoveAll(v.Mountpoint)
		if err != nil {
			return err
		}
	}
	d.Lock()
	defer d.Unlock()
	delete(d.volumes, v.Name)
	return nil
}

func (d *Driver) unmountVolume(v *Volume) error {
	return nil
}

func (d *Driver) getFiler(alias string) (*Filer, error) {
	d.RLock()
	_, ok := d.filers[alias]
	d.RUnlock()
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
		d.Lock()
		defer d.Unlock()
		d.filers[alias] = socats
	}
	return d.filers[alias], nil
}

func (d *Driver) manage() {
	for {
		if _, err := os.Stat(savePath); err == nil {
			cmd := exec.Command("echo", savePath+" was found")
			cmd.Stdout = d.Stdout
			cmd.Run()

			data, err := ioutil.ReadFile(savePath)
			if err != nil {
				logrus.WithField("loadDriver", savePath).Error(err)
			}
			var volumes []Volume
			json.Unmarshal(data, volumes)
			for _, v := range volumes {
				cmd := exec.Command("echo", v.Name+" was saved")
				cmd.Stdout = d.Stdout
				cmd.Run()

				d.RLock()
				vol := d.volumes[v.Name]
				d.RUnlock()
				if vol == nil {
					cmd := exec.Command("echo", vol.Name+" doesn't exist, creating")
					cmd.Stdout = d.Stdout
					cmd.Run()

					d.createVolume(&volume.CreateRequest{
						Name:    v.Name,
						Options: v.Options,
					})
				} else {
					cmd := exec.Command("echo", vol.Name+" already exists")
					cmd.Stdout = d.Stdout
					cmd.Run()
				}
			}
		} else {
			cmd := exec.Command("echo", err.Error())
			cmd.Stdout = d.Stdout
			cmd.Run()
		}
		time.Sleep(5 * time.Second)
	}
}
