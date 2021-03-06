package main

import (
	"errors"
	"os"
	"syscall"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const dockerSocket = "/run/docker/plugins/volumedriver.sock"
const seaweedfsSockets = "/var/lib/docker/plugins/seaweedfs"

var Stdout = os.NewFile(uintptr(syscall.Stdout), "/run/docker/plugins/init-stdout")
var Stderr = os.NewFile(uintptr(syscall.Stderr), "/run/docker/plugins/init-stderr")
var SeaweedFS = new(Driver)

func main() {
	err := SeaweedFS.init()
	if err != nil {
		logrus.Error(err)
	} else {
		h := volume.NewHandler(SeaweedFS)
		logrus.Infof("listening on %s", dockerSocket)
		logrus.Error(h.ServeUnix(dockerSocket, 0))
	}
}

// Get the list of capabilities the driver supports.
// The driver is not required to implement Capabilities. If it is not implemented, the default values are used.
func (d *Driver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}

// Create Instructs the plugin that the user wants to create a volume,
// given a user specified volume name. The plugin does not need to actually
// manifest the volume on the filesystem yet (until Mount is called).
// Opts is a map of driver specific options passed through from the user request.
func (d *Driver) Create(r *volume.CreateRequest) error {
	if err := d.createVolume(r); err != nil {
		return err
	}
	return nil
}

// Get info about volume_name.
func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	v, err := d.getVolume(r.Name)
	if err != nil {
		return &volume.GetResponse{}, err
	}
	return &volume.GetResponse{Volume: &volume.Volume{
		Name:       v.Name,
		Mountpoint: v.Mountpoint,
		Status:     v.getStatus(),
	}}, nil
}

// List of volumes registered with the plugin.
func (d *Driver) List() (*volume.ListResponse, error) {
	return &volume.ListResponse{Volumes: d.listVolumes()}, nil
}

// Mount is called once per container start.
// If the same volume_name is requested more than once, the plugin may need to keep
// track of each new mount request and provision at the first mount request and
// deprovision at the last corresponding unmount request.
// Docker requires the plugin to provide a volume, given a user specified volume name.
// ID is a unique ID for the caller that is requesting the mount.
func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	v, err := d.getVolume(r.Name)
	if err != nil {
		return &volume.MountResponse{}, err
	}
	err = v.Mount()
	if err != nil {
		return &volume.MountResponse{}, err
	}
	return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
}

// Path requests the path to the volume with the given volume_name.
func (d *Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	v, err := d.getVolume(r.Name)
	if err != nil {
		return &volume.PathResponse{}, err
	}
	return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil

}

// Remove the specified volume from disk. This request is issued when a
// user invokes docker rm -v to remove volumes associated with a container.
func (d *Driver) Remove(r *volume.RemoveRequest) error {
	if v, found := d.Volumes[r.Name]; found {
		err := d.removeVolume(v)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("volume " + r.Name + " not found")
	}
}

// Docker is no longer using the named volume.
// Unmount is called once per container stop.
// Plugin may deduce that it is safe to deprovision the volume at this point.
// ID is a unique ID for the caller that is requesting the mount.
func (d *Driver) Unmount(r *volume.UnmountRequest) error {
	if v, found := d.Volumes[r.Name]; found {
		return v.Unmount()
	} else {
		return errors.New("volume " + r.Name + " not found")
	}
}
