package main

import (
	"path/filepath"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

type volumeDriver struct {
	propagatedMount string
	volumes         map[string]*dockerVolume
}

func newVolumeDriver(propagatedMount string) (*volumeDriver, error) {
	logrus.WithField("method", "new driver").Debug(propagatedMount)
	d := &volumeDriver{
		propagatedMount: propagatedMount,
		volumes:         map[string]*dockerVolume{},
	}
	return d, nil
}

// Get the list of capabilities the driver supports.
// The driver is not required to implement Capabilities. If it is not implemented, the default values are used.
func (d *volumeDriver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "global"}}
}

// Create Instructs the plugin that the user wants to create a volume,
// given a user specified volume name. The plugin does not need to actually
// manifest the volume on the filesystem yet (until Mount is called).
// Opts is a map of driver specific options passed through from the user request.
func (d *volumeDriver) Create(r *volume.CreateRequest) error {
	logrus.WithField("method", "create").Debugf("%#v", r)
	v := &dockerVolume{
		Name:       r.Name,
		Mountpoint: filepath.Join(d.propagatedMount, r.Name),
		Options:    r.Options,
	}
	if err := d.updateVolume(v); err != nil {
		return err
	}
	return nil
}

// Get info about volume_name.
func (d *volumeDriver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	logrus.WithField("method", "get").Debugf("%#v", r)
	if v, found := d.volumes[r.Name]; found {
		return &volume.GetResponse{Volume: &volume.Volume{
			Name:       v.Name,
			Mountpoint: v.Mountpoint, // "/path/under/PropogatedMount"
		}}, nil
	} else {
		return &volume.GetResponse{}, logError("volume %s not found", r.Name)
	}
}

// List of volumes registered with the plugin.
func (d *volumeDriver) List() (*volume.ListResponse, error) {
	var vols = d.listVolumes()
	return &volume.ListResponse{Volumes: vols}, nil
}

// Mount is called once per container start.
// If the same volume_name is requested more than once, the plugin may need to keep
// track of each new mount request and provision at the first mount request and
// deprovision at the last corresponding unmount request.
// Docker requires the plugin to provide a volume, given a user specified volume name.
// ID is a unique ID for the caller that is requesting the mount.
func (d *volumeDriver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	logrus.WithField("method", "mount").Debugf("%#v", r)
	if v, found := d.volumes[r.Name]; found {
		d.mountVolume(v)
		return &volume.MountResponse{Mountpoint: v.Mountpoint}, nil
	} else {
		return &volume.MountResponse{}, logError("volume %s not found", r.Name)
	}
}

// Path requests the path to the volume with the given volume_name.
func (d *volumeDriver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	logrus.WithField("method", "path").Debugf("%#v", r)
	if v, found := d.volumes[r.Name]; found {
		return &volume.PathResponse{Mountpoint: v.Mountpoint}, nil
	} else {
		return &volume.PathResponse{}, logError("volume %s not found", r.Name)
	}

}

// Remove the specified volume from disk. This request is issued when a
// user invokes docker rm -v to remove volumes associated with a container.
func (d *volumeDriver) Remove(r *volume.RemoveRequest) error {
	logrus.WithField("method", "remove").Debugf("%#v", r)
	if v, found := d.volumes[r.Name]; found {
		err := d.unmountVolume(v)
		if err != nil {
			return logError(err.Error())
		}
		err = d.removeVolume(v)
		if err != nil {
			return logError(err.Error())
		}
		return nil
	} else {
		return logError("volume %s not found", r.Name)
	}
}

// Docker is no longer using the named volume.
// Unmount is called once per container stop.
// Plugin may deduce that it is safe to deprovision the volume at this point.
// ID is a unique ID for the caller that is requesting the mount.
func (d *volumeDriver) Unmount(r *volume.UnmountRequest) error {
	logrus.WithField("method", "unmount").Debugf("%#v", r)
	if v, found := d.volumes[r.Name]; found {
		v.Connections--
		if err := d.updateVolume(v); err != nil {
			logrus.WithField("updateVolume ERROR", err).Errorf("%#v", v)
		} else {
			logrus.WithField("updateVolume", r.Name).Debugf("%#v", v)
		}
		if v.Connections <= 0 {
			v.Connections = 0
			d.updateVolume(v)
			d.unmountVolume(v)
		}
		return nil
	} else {
		return logError("volume %s not found", r.Name)
	}
}
