package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

// mostly swiped from https://github.com/vieux/docker-volume-sshfs/blob/master/main.go
const socketAddress = "/run/docker/plugins/volumedriver.sock"
const propagatedMount = "/mnt"

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

	d, err := newVolumeDriver(propagatedMount)
	if err != nil {
		log.Fatal(err)
	}
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))
}
