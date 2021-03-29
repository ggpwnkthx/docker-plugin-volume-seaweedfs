package main

import (
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/volumedriver.sock"
const savePath = "/run/docker/plugins/volumedriver"

func main() {
	d := loadDriver()
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))

	//go d.manage()
}
