package main

import (
	"errors"
	"os"
	"reflect"

	weed "github.com/chrislusf/seaweedfs/weed/command"
	"github.com/docker/go-plugins-helpers/volume"
)

type dockerVolume struct {
	Options          map[string]string
	Name, Mountpoint string
	Connections      int
}

func (d *volumeDriver) listVolumes() []*volume.Volume {
	var volumes []*volume.Volume
	for _, mount := range d.volumes {
		var v volume.Volume
		v.Name = mount.Name
		v.Mountpoint = mount.Mountpoint
		volumes = append(volumes, &v)
	}
	return volumes
}

func (d *volumeDriver) mountVolume(v *dockerVolume) error {
	v.Connections++
	return nil
}

func (d *volumeDriver) removeVolume(v *dockerVolume) error {
	if v.Connections == 0 {
		err := os.RemoveAll(v.Mountpoint)
		if err != nil {
			return err
		}
		delete(d.volumes, v.Name)
		return nil
	} else {
		return errors.New("Active connections still exist.")
	}
}

func (d *volumeDriver) unmountVolume(v *dockerVolume) error {
	v.Connections--
	return nil
}

func (d *volumeDriver) updateVolume(v *dockerVolume) error {
	if _, found := d.volumes[v.Name]; found {
		d.volumes[v.Name] = v
	} else {
		if _, err := os.Stat(v.Mountpoint); err != nil {
			if os.IsNotExist(err) {
				os.MkdirAll(v.Mountpoint, 760)
			}
		}
		mOptions := weed.MountOptions{
			allowOthers:        true,
			dir:                v.Mountpoint,
			dirAutoCreate:      true,
			volumeServerAccess: "filerProxy",
		}
		for oKey, oValue := range v.Options {
			structValue := reflect.ValueOf(mOptions).Elem()
			structFieldValue := structValue.FieldByName(oKey)
			if !structFieldValue.IsValid() {
				continue
			}
			if !structFieldValue.CanSet() {
				continue
			}
			structFieldType := structFieldValue.Type()
			val := reflect.ValueOf(oValue)
			if structFieldType != val.Type() {
				return errors.New("Provided value type didn't match obj field type")
			}
			structFieldValue.Set(val)
		}
		weed.RunMount(mOptions)
		d.volumes[v.Name] = v
	}
	return nil
}
