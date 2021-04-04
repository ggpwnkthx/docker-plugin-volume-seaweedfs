package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	client http.Client
	http   *Socat
	grpc   *Socat
}

var Filers = struct {
	sync.RWMutex
	list map[string]*Filer
}{
	list: map[string]*Filer{},
}

func (f *Filer) listVolumes() (*[]volume.CreateRequest, error) {
	var volumes []volume.CreateRequest
	data, err := f.getFile("volumes.json")
	if err != nil {
		return &volumes, err
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
	f.setFile("volumes.json", data)
	return nil
}
func (f *Filer) getFile(path string) ([]byte, error) {
	response, err := f.client.Get("http://localhost/" + path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
func (f *Filer) setFile(path string, data []byte) error {
	filename := strings.Split(path, "/")[len(strings.Split(path, "/"))-1]

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	mw := multipart.NewWriter(writer)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	mtype := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if mtype != "" {
		header.Set("Content-Type", mtype)
	}
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(data)

	if err == nil {
		if err = mw.Close(); err == nil {
			err = writer.Close()
		} else {
			_ = writer.Close()
		}
	} else {
		_ = mw.Close()
		_ = writer.Close()
	}
	if err != nil {
		return err
	}

	response, err := f.client.Post(path, mw.FormDataContentType(), reader)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return errors.New(string(content))
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

	filer.client = http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", filer.http.Sock)
			},
		},
	}

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
