package main

import (
	"errors"
	"io"
	"net"
	"strconv"

	"github.com/phayes/freeport"
)

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