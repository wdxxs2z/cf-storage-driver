# cf-storage-driver
### Run It
Run voldriver
```
sudo voldriver -volmanDriverPaths /tmp/voldriver
```
Run cf-storage-driver
```
sudo ./nfsdriver -driversPath /tmp/voldriver
```
More /tmp/voldriver/nfsdriver.json
```
{"Name":"nfsdriver","Addr":"http://0.0.0.0:5566","TLSConfig":null}
```
### Http Rest Client Test
Get http://{voldriverAddr}:8750/drivers
```
{"drivers": [{"name": "nfsdriver"}]}
```
Post http://{voldriverAddr}:8750/drivers/mount
```
{"driverId":"nfsdriver","volumeId":"/tmp/docker","config":{"remoteinfo":"10.10.130.57","version":3.0,"remotemountpoint":"/var/vcap/store","localmountpoint":"/tmp/docker","opts":"port=2049,nolock,proto=tcp"}}
```
Post Mount Response JSON
```
{"timestamp":"1466408395.876440048","source":"nfs-driver-server","message":"nfs-driver-server.server.handle-create.create.duplicate-volume","log_level":1,"data":{"request":{"Name":"/tmp/docker","Opts":{"localmountpoint":"/tmp/docker","opts":"port=2049,nolock,proto=tcp","remoteinfo":"10.10.130.57","remotemountpoint":"/var/vcap/store","version":3}},"session":"2.7.1","volume name":"/tmp/docker"}}
```
Post http://{voldriverAddr}:8750/drivers/unmount
```
{"driverId":"nfsdriver","volumeId":"/tmp/docker","config": {localmountpoint":"/tmp/docker"}}
```

#### Can't resolved - important
Only use sudo (root) otherwise cause some permission errors.
