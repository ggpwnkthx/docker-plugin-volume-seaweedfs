package main

import (
	"bufio"
	"errors"
	"os/exec"
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

func Contains(haystack []string, needle string) bool {
	for _, test := range haystack {
		if test == needle {
			return true
		}
	}
	return false
}

func SeaweedFSMount(cmd *exec.Cmd, options []string) {
	logerr(options...)
	if cmd == nil {
		cmd = exec.Command("/usr/bin/weed", options...)
	}
	stderr, _ := cmd.StderrPipe()
	cmd.Start()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "mounted localhost") {
				break
			}
		}
	}()
	wg.Wait()
}
