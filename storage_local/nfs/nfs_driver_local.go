package storage_nfsdriver

import (
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	osshim "code.cloudfoundry.org/goshims/os"
	"code.cloudfoundry.org/goshims/execshim"
	"code.cloudfoundry.org/goshims/ioutil"

	"strings"
	"fmt"
	"time"
)

const (
	Name = "nfs"
)

type NfsLocalDriver struct {
	rootDir          string
	logFile          string
	volumes          map[string]*volumeMetadata
	userInvoker      Invoker
	os               osshim.Os
	useSystemUtil    ioutilshim.Ioutil
}

type volumeMetadata struct {
	RemoteInfo       string
	RemoteMountPoint string
	LocalMountPoint  string
	Version          float32
	Opts             string
	MountCount       int
}

func NewNfsLocalDriver() *NfsLocalDriver {
	return NewLocalDriverWithSystemUtilAndInvoker(&ioutilshim.IoutilShim{}, &osshim.OsShim{} , NewRealInvoker())
}

func NewLocalDriverWithSystemUtilAndInvoker(ioutil ioutilshim.Ioutil, os osshim.Os, invoker Invoker) *NfsLocalDriver {
	return &NfsLocalDriver{
		"_nfsdriver/",
		"/tmp/nfsdriver.log",
		map[string]*volumeMetadata{},
		invoker,
		os,
		ioutil,
	}
}

func (d *NfsLocalDriver) Create(logger lager.Logger, createRequest voldriver.CreateRequest) voldriver.ErrorResponse {
	logger = logger.Session("create", lager.Data{"request": createRequest})
	logger.Info("start")
	defer logger.Info("end")

	var (
		localmountpoint  string
		remotemountpoint string
		remoteinfo       string
		opts             string
		version          float32
		err              *voldriver.ErrorResponse
	)

	localmountpoint, err = extractValue(logger, "localmountpoint", createRequest.Opts)
	if err != nil {
		return *err
	}
	remotemountpoint, err = extractValue(logger, "remotemountpoint", createRequest.Opts)
	if err != nil {
		return *err
	}
	remoteinfo, err = extractValue(logger, "remoteinfo", createRequest.Opts)
	if err != nil {
		return *err
	}
	opts, err = extractValue(logger, "opts", createRequest.Opts)
	if err != nil {
		return *err
	}
	return d.create(logger, createRequest.Name, remoteinfo, remotemountpoint, localmountpoint, version, opts)
}

func (d *NfsLocalDriver) create(logger lager.Logger, name, remoteinfo, remotemountpoint, localmountpoint  string, version float32, opts string) voldriver.ErrorResponse {
	var volume *volumeMetadata
	var ok     bool

	newVolume := &volumeMetadata{
		RemoteInfo:          remoteinfo,
		RemoteMountPoint:    remotemountpoint,
		LocalMountPoint:     localmountpoint,
		Version:             version,
		Opts:                opts,
	}

	if volume, ok = d.volumes[name]; !ok {
		logger.Info("create-volume", lager.Data{"volume name" : name})
		d.volumes[name] = newVolume
		return successfulResponse()
	}

	if volume.equals(newVolume) {
		logger.Info("duplicate-volume", lager.Data{"volume name" : name})
		return successfulResponse()
	}

	logger.Info("duplicate-volume-with-different-opts", lager.Data{"volume_name": name, "existing-volume": volume})
	return voldriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' already exists with different Opts", name)}
}

func (v *volumeMetadata) equals(volume *volumeMetadata) bool {
	return volume.LocalMountPoint == v.LocalMountPoint && volume.RemoteMountPoint == v.RemoteMountPoint && volume.RemoteInfo == v.RemoteInfo
}

func successfulResponse() voldriver.ErrorResponse {
	return voldriver.ErrorResponse{}
}

func (d *NfsLocalDriver) Get(logger lager.Logger, getRequest voldriver.GetRequest) voldriver.GetResponse {
	logger.Session("Get")
	logger.Info("start")
	defer logger.Info("end")

	if volume, ok := d.volumes[getRequest.Name]; ok {
		logger.Info("get-nfs-volume", lager.Data{"volume_name" : getRequest.Name})
		if volume.MountCount > 0 {
			return voldriver.GetResponse{Volume: voldriver.VolumeInfo{
				Name:          getRequest.Name,
				Mountpoint:    volume.LocalMountPoint,
			}}
		}
		return voldriver.GetResponse{Volume: voldriver.VolumeInfo{Name: getRequest.Name}}
	}
	logger.Info("get-nfs-volume-not-found", lager.Data{"volume_name" : getRequest.Name})
	return voldriver.GetResponse{Err: fmt.Sprintf("Volume %s not found", getRequest.Name)}
}

func (d *NfsLocalDriver) Path(logger lager.Logger, getRequest voldriver.PathRequest) voldriver.PathResponse {
	logger.Session("Path")
	logger.Info("start")
	defer logger.Info("end")

	if volume, ok := d.volumes[getRequest.Name] ; ok {
		if volume.MountCount > 0 {
			logger.Info("nfs-volume-path", lager.Data{"volume_name": getRequest.Name, "volume_path": volume.LocalMountPoint})
			return voldriver.PathResponse{Mountpoint:volume.LocalMountPoint}
		}
		logger.Info("nfs-volume-path-not-mounted",lager.Data{"volume_name": getRequest.Name})
		return voldriver.PathResponse{Err: fmt.Sprintf("Volume %s are not mounted",getRequest.Name)}
	}
	logger.Info("nfs-volume-path-not-found",lager.Data{"volume_name": getRequest.Name})
	return voldriver.PathResponse{Err: fmt.Sprintf("Volume %s are not found",getRequest.Name)}
}

func (d *NfsLocalDriver) List(logger lager.Logger) voldriver.ListResponse {
	logger.Session("List")
	logger.Info("start")
	defer logger.Info("end")

	volinfo := voldriver.VolumeInfo{}
	listResponse := voldriver.ListResponse{}

	for v_name,volume := range d.volumes {
		volinfo.Name = v_name
		if volinfo.MountCount > 0 {
			volinfo.Mountpoint = volume.LocalMountPoint
		} else {
			volinfo.Mountpoint = ""
		}
		listResponse.Volumes = append(listResponse.Volumes,volinfo)
	}
	listResponse.Err = ""
	return listResponse
}

func (d *NfsLocalDriver) Mount(logger lager.Logger, mountRequest voldriver.MountRequest) voldriver.MountResponse {
	logger.Session("Mount")
	logger.Info("start")
	defer logger.Info("end")

	var volume *volumeMetadata
	var ok     bool

	if volume, ok = d.volumes[mountRequest.Name]; !ok {
		logger.Info("mount-volume-not-found",lager.Data{"volume_name": mountRequest.Name})
		return voldriver.MountResponse{Err: fmt.Sprintf("Volume '%s' not found", mountRequest.Name)}
	}

	if volume.MountCount > 0 {
		volume.MountCount ++
		logger.Info("mount-volume-already-mounted", lager.Data{"volume": volume})
		return voldriver.MountResponse{Mountpoint:volume.LocalMountPoint}
	}

	err := d.os.MkdirAll(volume.LocalMountPoint, os.ModePerm)
	if err != nil {
		logger.Error("failed-create-mountdir",err)
		return voldriver.MountResponse{Err: fmt.Sprintf("unable to create local mount point '%s'", mountRequest.Name)}
	}

	//Judgement the nfs version
	var cmdArgs []string
	switch volume.Version {
	case 3.0:
		if len(volume.Opts) < 1 {
			cmdArgs = []string{"-t", "nfs", "-o", "port=2049,nolock,proto=tcp", volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		} else {
			cmdArgs = []string{"-t", "nfs", "-o", volume.Opts, volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		}
	case 4.1,4.0:
		if len(volume.Opts) < 1 {
			cmdArgs = []string{"-t", "nfs4" , "-o", "vers=4,minorversion=1", volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		} else {
			cmdArgs = []string{"-t", "nfs4" , "-o", volume.Opts, volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		}
	default:
		if len(volume.Opts) < 1 {
			cmdArgs = []string{"-o", "nolock", volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		} else {
			cmdArgs = []string{"-o", volume.Opts, volume.RemoteInfo + ":" + volume.RemoteMountPoint, volume.LocalMountPoint}
		}
	}

	tryTimes := 0
	retry:
	if err := d.invokeNFS(logger, cmdArgs); err !=  nil {
		logger.Error("Error mounting volume, trying mount " + string(tryTimes) + " times", err)
		time.Sleep(time.Second)
		tryTimes++
		if tryTimes == 3 {
			return voldriver.MountResponse{Err: fmt.Sprintf("Error mounting '%s' (%s)", mountRequest.Name, err.Error())}
		}
		goto retry
	}

	volume.MountCount = 1
	return voldriver.MountResponse{Mountpoint: volume.LocalMountPoint}
}

func (d *NfsLocalDriver) Unmount(logger lager.Logger, unmountRequest voldriver.UnmountRequest) voldriver.ErrorResponse  {
	logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")

	var volume *volumeMetadata
	var ok     bool

	if volume, ok = d.volumes[unmountRequest.Name]; !ok {
		logger.Info("unmount-volume-not-found",lager.Data{"volume_name": unmountRequest.Name})
		return voldriver.ErrorResponse{Err: fmt.Sprintf("Volume '%s' not found", unmountRequest.Name)}
	}
	if volume.MountCount == 0 {
		logger.Info("unmount-volume-not-mounted", lager.Data{"volume_name": unmountRequest.Name})
		return voldriver.ErrorResponse{Err: fmt.Sprintf("volume '%s' not found", unmountRequest.Name)}
	}

	return d.unmount(logger, volume, unmountRequest.Name)
}

func (d *NfsLocalDriver) unmount(logger lager.Logger, volume *volumeMetadata, volumeName string) voldriver.ErrorResponse {
	logger.Info("umount-found-volume", lager.Data{"metadata": volume})

	if volume.MountCount > 1 {
		volume.MountCount--
		logger.Info("unmount-volune-in-use", lager.Data{"metadata": volume})
		return voldriver.ErrorResponse{Err: fmt.Sprintf("volume '%s' maybe in use", volumeName)}
	}

	cmdArgs := []string{volume.LocalMountPoint}
	if err := d.userInvoker.Invoke(logger, "umount", cmdArgs); err != nil {
		logger.Error("Error invoking unmount cli", err)
		return voldriver.ErrorResponse{Err: fmt.Sprintf("Error unmount '%s' (%s)",volumeName, err.Error())}
	}

	volume.MountCount = 0
	if err := d.os.Remove(volume.LocalMountPoint); err != nil {
		logger.Error("Error deleting file", err)
		return voldriver.ErrorResponse{Err: fmt.Sprintf("Error unmount '%s' (%s)",volumeName, err.Error())}
	}

	return voldriver.ErrorResponse{}
}

func (d *NfsLocalDriver) Remove(logger lager.Logger, removeRequest voldriver.RemoveRequest) voldriver.ErrorResponse {
	logger.Session("remove", lager.Data{"volume": removeRequest})
	logger.Info("start")
	defer logger.Info("end")

	if removeRequest.Name == "" {
		return voldriver.ErrorResponse{Err: "Missing metadata 'volume name'"}
	}

	var response     voldriver.ErrorResponse
	var volume       *volumeMetadata
	var exists       bool

	if volume, exists = d.volumes[removeRequest.Name]; !exists {
		logger.Error("failed-volume-remove", fmt.Errorf(fmt.Sprintf("Volume %s not found", removeRequest.Name)))
		return voldriver.ErrorResponse{fmt.Sprintf("volume '%s' not found", removeRequest.Name)}
	}

	for ; volume.MountCount > 0 ;{
		response = d.unmount(logger, volume, removeRequest.Name)
		if response.Err != "" {
			return response
		}
	}

	logger.Info("removing-volume", lager.Data{"volume_name": removeRequest.Name})
	delete(d.volumes, removeRequest.Name)
	return voldriver.ErrorResponse{}
}

func (d *NfsLocalDriver) Activate(logger lager.Logger) voldriver.ActivateResponse {

	return voldriver.ActivateResponse{
		Implements: []string{"VolumeDriver"},
	}
}

func (d *NfsLocalDriver) Capabilities(logger lager.Logger) voldriver.CapabilitiesResponse {
	return voldriver.CapabilitiesResponse{
		Capabilities: voldriver.CapabilityInfo{Scope: "global"},
	}
}

func (d *NfsLocalDriver) invokeNFS(logger lager.Logger, args []string) error {
	cmd := "mount"
	return d.userInvoker.Invoke(logger, cmd, args)
}

func extractValue(logger lager.Logger, value string, opts map[string]interface{}) (string, *voldriver.ErrorResponse) {
	var aString interface{}
	var str     string
	var ok      bool

	if aString, ok = opts[value]; !ok {
		logger.Info("missing-" + strings.ToLower(value))
		return "", &voldriver.ErrorResponse{Err: fmt.Sprintf("Missing Mandatory '%s' field in Opts", value)}
	}
	if str, ok = aString.(string); !ok {
		logger.Info("missing-" + strings.ToLower(value))
		return "", &voldriver.ErrorResponse{Err: fmt.Sprintf("Unable to string convert '%s' field in Opts", value)}
	}
	return str, nil
}

type Invoker interface {
	Invoke(logger lager.Logger, executable string, args []string) error
}

type realInvoker struct {
	exec execshim.Exec
}

func NewRealInvoker() Invoker {
	return NewRealInvokerWithExec(&execshim.ExecShim{})
}

func NewRealInvokerWithExec(exec execshim.Exec) Invoker {
	return &realInvoker{
		exec:        exec,
	}
}

func (r *realInvoker) Invoke(logger lager.Logger, executable string, args []string) error {
	cmdHandle := r.exec.Command(executable, args...)

	_,err := cmdHandle.StdoutPipe()
	if err != nil {
		logger.Error("unable to get stdout", err)
		return err
	}

	err = cmdHandle.Start()
	if err != nil {
		logger.Error("start command error", err)
		return err
	}

	err = cmdHandle.Wait()
	if err != nil {
		logger.Error("wait command error", err)
		return err
	}

	return nil
}