package main

import (
	"errors"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/volumedriver.sock"
const savePath = "/var/lib/docker/plugins/seaweedfs/volumes.json"

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
	if err := CreateVolume(d, r); err != nil {
		return err
	}
	return nil
}

// Get info about volume_name.
func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	if v, found := d.volumes[r.Name]; found {
		return &volume.GetResponse{Volume: &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint,
			Status:     v.getStatus(),
		}}, nil
	} else {
		return &volume.GetResponse{}, errors.New("volume " + r.Name + " not found")
	}
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
	if v, found := d.volumes[r.Name]; found {
		v.Mount()
		return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
	} else {
		return &volume.MountResponse{}, errors.New("volume " + r.Name + " not found")
	}
}

// Path requests the path to the volume with the given volume_name.
func (d *Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	if v, found := d.volumes[r.Name]; found {
		return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
	} else {
		return &volume.PathResponse{}, errors.New("volume " + r.Name + " not found")
	}

}

// Remove the specified volume from disk. This request is issued when a
// user invokes docker rm -v to remove volumes associated with a container.
func (d *Driver) Remove(r *volume.RemoveRequest) error {
	if v, found := d.volumes[r.Name]; found {
		err := v.Remove()
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
	if v, found := d.volumes[r.Name]; found {
		return v.Unmount()
	} else {
		return errors.New("volume " + r.Name + " not found")
	}
}

func main() {
	d := loadDriver()
	h := volume.NewHandler(d)
	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, 0))
}
