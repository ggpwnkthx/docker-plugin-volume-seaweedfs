package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/phayes/freeport"
)

type Socat struct {
	Cmd  *exec.Cmd
	Port int
	Sock string
}

type Filer struct {
	http *Socat
	grpc *Socat
}

var Filers = struct {
	sync.RWMutex
	list map[string]*Filer
}{}

func getFiler(alias string) (*Filer, error) {
	Filers.RLock()
	_, ok := Filers.list[alias]
	Filers.RUnlock()
	if !ok {
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, alias), os.ModeDir)
		port := 0
		for {
			port, err := freeport.GetFreePort()
			if err != nil {
				return &Filer{}, errors.New("freeport: " + err.Error())
			}
			if port != 0 && port < 55535 {
				break
			}
		}

		filer := &Filer{
			http: &Socat{
				Port: port,
				Sock: filepath.Join(seaweedfsSockets, alias, "http.sock"),
			},
			grpc: &Socat{
				Port: port + 10000,
				Sock: filepath.Join(seaweedfsSockets, alias, "grpc.sock"),
			},
		}
		if _, err := os.Stat(filer.http.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("http unix socket not found")
		}
		if _, err := os.Stat(filer.grpc.Sock); os.IsNotExist(err) {
			return &Filer{}, errors.New("grpc unix socket not found")
		}

		httpOptions := []string{
			"-d",
			"tcp-l:" + strconv.Itoa(filer.http.Port) + ",fork",
			"unix:" + filer.http.Sock,
		}
		filer.http.Cmd = exec.Command("/usr/bin/socat", httpOptions...)
		filer.http.Cmd.Stderr = Stderr
		filer.http.Cmd.Stdout = Stdout
		filer.http.Cmd.Start()

		grpcOptions := []string{
			"-d",
			"tcp-l:" + strconv.Itoa(filer.grpc.Port) + ",fork",
			"unix:" + filer.grpc.Sock,
		}
		filer.grpc.Cmd = exec.Command("/usr/bin/socat", grpcOptions...)
		filer.grpc.Cmd.Stderr = Stderr
		filer.grpc.Cmd.Stdout = Stdout
		filer.grpc.Cmd.Start()

		setFiler(alias, filer)
	}
	return Filers.list[alias], nil
}
func setFiler(alias string, filer *Filer) {
	Filers.Lock()
	defer Filers.Unlock()
	Filers.list[alias] = filer
}
