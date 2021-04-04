package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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
	alias string
	weed  *exec.Cmd
	http  *Socat
	grpc  *Socat
}

var Filers = struct {
	sync.RWMutex
	list map[string]*Filer
}{
	list: map[string]*Filer{},
}

func (f *Filer) listVolumes() (*[]volume.CreateRequest, error) {
	var volumes []volume.CreateRequest
	path := filepath.Join("/mnt", f.alias, "volumes.json")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return &volumes, nil
	}
	if data == nil {
		return &volumes, nil
	}
	json.Unmarshal(data, &volumes)
	return &volumes, nil
}
func (f *Filer) saveVolumes(v []volume.CreateRequest) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	path := filepath.Join("/mnt", f.alias, "volumes.json")
	ioutil.WriteFile(path, data, 0644)
	return nil
}

func isFiler(alias string) bool {
	Filers.RLock()
	defer Filers.RUnlock()
	_, ok := Filers.list[alias]
	return ok
}
func getFiler(alias string) (*Filer, error) {
	if !isFiler(alias) {
		logerr("alias " + alias + " doesn't exists, creating it")
		createFiler(alias)
	}
	return Filers.list[alias], nil
}
func createFiler(alias string) error {
	port, err := getFreePort()
	if err != nil {
		return err
	}
	logerr("using port " + strconv.Itoa(port))

	filer := &Filer{
		alias: alias,
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
		return errors.New("http unix socket not found")
	}
	if _, err := os.Stat(filer.grpc.Sock); os.IsNotExist(err) {
		return errors.New("grpc unix socket not found")
	}

	go gocat_tcp2unix(filer.http.Port, filer.http.Sock)
	go gocat_tcp2unix(filer.grpc.Port, filer.grpc.Sock)

	mountpoint := filepath.Join("/mnt", alias)
	os.MkdirAll(mountpoint, os.ModePerm)

	mOptions := []string{
		"mount",
		"-dir=" + mountpoint,
		"-filer=localhost:" + strconv.Itoa(filer.http.Port),
		"-volumeServerAccess=filerProxy",
	}
	filer.weed = exec.Command("/usr/bin/weed", mOptions...)
	filer.weed.Stderr = Stderr
	filer.weed.Stdout = Stdout
	filer.weed.Start()

	setFiler(alias, filer)

	return nil
}
func setFiler(alias string, filer *Filer) {
	Filers.Lock()
	defer Filers.Unlock()
	Filers.list[alias] = filer
}
func availableFilers() ([]string, error) {
	dirs := []string{}
	items, err := ioutil.ReadDir(seaweedfsSockets)
	if err != nil {
		return []string{}, err
	}
	for _, i := range items {
		if i.IsDir() {
			dirs = append(dirs, i.Name())
		}
	}
	return dirs, nil
}

func getFreePort() (int, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0, errors.New("freeport: " + err.Error())
	}
	if port == 0 || port > 55535 {
		return getFreePort()
	}
	return port, nil
}

func gocat_tcp2unix(port int, socketPath string) {
	for {
		l, err := net.Listen("tcp", "localhost:"+strconv.Itoa(port))
		if err != nil {
			logerr(err.Error())
			return
		}
		for {
			tconn, err := l.Accept()
			if err != nil {
				logerr(err.Error())
				continue
			}
			go gocat_forward2unix(tconn, socketPath)
		}
	}
}
func gocat_forward2unix(tconn net.Conn, socketPath string) {
	defer tconn.Close()
	uconn, err := net.Dial("unix", socketPath)
	if err != nil {
		logerr(err.Error())
		return
	}
	go io.Copy(tconn, uconn)
	io.Copy(uconn, tconn)
}
