package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

// mostly swiped from https://github.com/vieux/docker-volume-sshfs/blob/master/main.go
const socketAddress = "/run/docker/plugins/volumedriver.sock"

// Error log helper
func logError(format string, args ...interface{}) error {
	logrus.Errorf(format, args...)
	return fmt.Errorf(format, args...)
}

func main() {
	debug := os.Getenv("DEBUG")
	if ok, _ := strconv.ParseBool(debug); ok {
		logrus.SetLevel(logrus.DebugLevel)
	}

	d := &Driver{
		filers:      map[string]*Filer{},
		socketMount: "/var/lib/docker/plugins/seaweedfs/",
		Stdout:      os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout"),
		Stderr:      os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr"),
		volumes:     map[string]*Volume{},
	}
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))

	//go d.manage()
}
