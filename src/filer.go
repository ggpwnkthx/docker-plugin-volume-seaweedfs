package main

import (
	"errors"
	"io"
	"net"
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
}{
	list: map[string]*Filer{},
}

func getFreePort() (int, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0, errors.New("freeport: " + err.Error())
	}
	if port != 0 && port < 55535 {
		return getFreePort()
	}
	return port, nil
}

func getFiler(alias string) (*Filer, error) {
	Filers.RLock()
	_, ok := Filers.list[alias]
	Filers.RUnlock()
	if !ok {
		logerr("alias " + alias + " doesn't exists")
		os.MkdirAll(filepath.Join(volume.DefaultDockerRootDirectory, alias), os.ModeDir)
		port, err := getFreePort()
		if err != nil {
			return &Filer{}, err
		}
		logerr("using port " + strconv.Itoa(port))

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
		/*
			// Use socat
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
		*/
		// Use io.copy
		go gocat_tcp2unix(filer.http.Port, filer.http.Sock)
		go gocat_tcp2unix(filer.grpc.Port, filer.grpc.Sock)

		setFiler(alias, filer)
	}
	return Filers.list[alias], nil
}
func setFiler(alias string, filer *Filer) {
	Filers.Lock()
	defer Filers.Unlock()
	Filers.list[alias] = filer
}

func gocat_tcp2unix(port int, socketPath string) {
	for {
		l, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
		if err != nil {
			logerr(err.Error())
			return
		}
		for {
			uconn, err := l.Accept()
			if err != nil {
				logerr(err.Error())
				continue
			}
			go gocat_forward2unix(uconn, socketPath)
		}
	}
}
func gocat_forward2unix(uconn net.Conn, socketPath string) {
	defer uconn.Close()
	tconn, err := net.Dial("unix", socketPath)
	if err != nil {
		logerr(err.Error())
		return
	}
	go io.Copy(uconn, tconn)
	io.Copy(tconn, uconn)
}
