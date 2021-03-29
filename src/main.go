package main

import (
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/volumedriver.sock"
const savePath = "/var/lib/docker/plugins/seaweedfs/volumes.json"

func main() {
	d := loadDriver()
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))

}
