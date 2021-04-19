package main

import (
	"bufio"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phayes/freeport"
)

func logerr(message ...string) {
	cmd := exec.Command("echo", message...)
	cmd.Stdout = Stderr
	cmd.Run()
}

func getFreePort() (int64, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0, errors.New("freeport: " + err.Error())
	}
	if port == 0 || port > 55535 {
		return getFreePort()
	}
	return int64(port), nil
}

func Contains(haystack []string, needle string) bool {
	for _, test := range haystack {
		if test == needle {
			return true
		}
	}
	return false
}

func isFiler(alias string) bool {
	http := filepath.Join(seaweedfsSockets, alias, "http.sock")
	if _, err := os.Stat(http); os.IsNotExist(err) {
		logerr("isFiler:", alias, "is missing http.sock")
		return false
	}
	grpc := filepath.Join(seaweedfsSockets, alias, "grpc.sock")
	if _, err := os.Stat(grpc); os.IsNotExist(err) {
		logerr("isFiler:", alias, "is missing grpc.sock")
		return false
	}
	logerr("isFiler:", alias, "is a filer")
	return true
}
func availableFilers() ([]string, error) {
	dirs := []string{}
	items, err := ioutil.ReadDir(seaweedfsSockets)
	if err != nil {
		return []string{}, err
	}
	for _, i := range items {
		if i.IsDir() {
			logerr("availableFilers:", "found dir", i.Name())
			dirs = append(dirs, i.Name())
		}
	}
	filers := []string{}
	for _, d := range dirs {
		if isFiler(d) {
			filers = append(filers, d)
		}
	}
	return filers, nil
}

func SeaweedFSMount(options []string) *exec.Cmd {
	logerr(options...)
	cmd := exec.Command("/usr/bin/weed", options...)
	stderr, _ := cmd.StderrPipe()
	err := cmd.Start()
	if err != nil {
		logerr(err.Error())
		return nil
	}
	logerr("mount started, waiting for stable connection")
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logerr(line)
		if strings.Contains(line, "mounted localhost") {
			break
		}
	}
	return cmd
}
