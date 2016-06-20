package nfsdriver_init

type MountConfig struct {
	RemoteInfo       string     `json:"remoteInfo"`
	Version          float32    `json:"version"`
	RemoteMountPoint string     `json:"remote_mountpoint"`
	Opts             string     `json:"opts"`
}
