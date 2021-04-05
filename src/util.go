package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

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

func Contains(haystack []string, needle string) bool {
	for _, test := range haystack {
		if test == needle {
			return true
		}
	}
	return false
}

func SeaweedFSMount(cmd *exec.Cmd, options []string) {
	if cmd == nil {
		cmd = exec.Command("/usr/bin/weed", options...)
	}
	cmd.Stderr = Stderr
	cmd.Stdout = Stdout
	cmd.Start()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go WaitForStdLine("mounted localhost", Stderr, &wg)
	wg.Wait()
}

func WaitForStdLine(needle string, haystack *os.File, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		line, _ := bufio.NewReader(haystack).ReadString('\n')
		logerr("scanning: " + line)
		if strings.Contains(line, needle) {
			break
		}
	}
}
