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
)

type Driver struct {
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
	FilerAlias, Mountpoint, Name string
	Options                      map[string]string
	weed                         *exec.Cmd
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	filer := strings.Split(r.Options["filer"], ":")
	delete(r.Options, "filer")

	f, err := d.getFiler(filer[0])
	if err != nil {
		return err
	}

	v := &Volume{
		FilerAlias: filer[0],
		Mountpoint: filepath.Join(volume.DefaultDockerRootDirectory, r.Name),
		Name:       r.Name,
		Options:    r.Options,
	}
	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=localhost:" + strconv.Itoa(f.http.Port),
		"-volumeServerAccess=filerProxy",
	}
	for oKey, oValue := range r.Options {
		if oValue != "" {
			mOptions = append(mOptions, "-"+oKey+"="+oValue)
		} else {
			mOptions = append(mOptions, "-"+oKey)
		}
	}
	v.weed = exec.Command("/usr/bin/weed", mOptions...)
	v.weed.Stderr = d.Stderr
	v.weed.Stdout = d.Stdout
	v.weed.Start()

	d.volumes[r.Name] = v

	return nil
}

func (d *Driver) getVolumeStatus(v *Volume) map[string]interface{} {
	status := make(map[string]interface{})
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
