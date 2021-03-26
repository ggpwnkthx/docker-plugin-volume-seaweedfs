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
	Mountpoint, Name string
	Options          map[string]string
	filer            *Filer
	weed             *exec.Cmd
}

func (d *Driver) createVolume(r *volume.CreateRequest) error {
	_, ok := r.Options["filer"]
	if !ok {
		return errors.New("no filer address:port specified")
	}
	filer := strings.Split(r.Options["filer"], ":")
	delete(r.Options, "filer")

	_, ok = d.filers[filer[0]]
	if !ok {
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, filer[0]), os.ModeDir)

		port, err := freeport.GetFreePort()
		if err != nil {
			return errors.New("freeport: " + err.Error())
		}
		http := &Socat{
			Port: port,
			Sock: filepath.Join(d.socketMount, filer[0], "http.sock"),
		}
		httpOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(http.Port) + ",fork",
			"unix:" + http.Sock,
		}
		http.Cmd = exec.Command("/usr/bin/socat", httpOptions...)
		http.Cmd.Stderr = d.Stderr
		http.Cmd.Stdout = d.Stdout
		d.filers[filer[0]].http = http
		d.filers[filer[0]].http.Cmd.Start()

		grpc := &Socat{
			Port: port + 10000,
			Sock: filepath.Join(d.socketMount, filer[0], "grpc.sock"),
		}
		grpcOptions := []string{
			"-d", "-d", "-d",
			"tcp-l:" + strconv.Itoa(grpc.Port) + ",fork",
			"unix:" + grpc.Sock,
		}
		grpc.Cmd = exec.Command("/usr/bin/socat", grpcOptions...)
		grpc.Cmd.Stderr = d.Stderr
		grpc.Cmd.Stdout = d.Stdout
		d.filers[filer[0]].grpc = grpc
		d.filers[filer[0]].grpc.Cmd.Start()
	}

	v := &Volume{
		Mountpoint: filepath.Join(volume.DefaultDockerRootDirectory, filer[0], r.Name),
		Options:    r.Options,
		Name:       r.Name,
		filer:      d.filers[filer[0]],
	}
	mOptions := []string{
		"mount",
		"-allowOthers",
		"-dir=" + v.Mountpoint,
		"-dirAutoCreate",
		"-filer=localhost:" + strconv.Itoa(v.filer.http.Port),
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
	status["filer"] = v.filer
	status["weed"] = v.weed
	cmd, _ := exec.Command("ls", v.Mountpoint).CombinedOutput()
	status["ls"] = string(cmd[:])
	return status
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
