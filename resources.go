package cf_nfsdriver

type MountConfig struct {
	RemoteInfo       string     `json:"remoteInfo"`
	Version          float32    `json:"version"`
	RemoteMountPoint string     `json:"remote_mountpoint"`
}
