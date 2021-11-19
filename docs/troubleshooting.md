# Troubleshooting

## Too many open files

If your relay logs errors similar to these:
```
2021-10-29 23:09:45.919 [ERROR] error accepting on 0.0.0.0:2003/tcp, closing connection: accept tcp [::]:2003: accept4: too many open files
```

Then the system has too many open files (file descriptors), ie network connections or handles to files on the filesystem.
These large amounts of FD's (file descriptors) may be held by carbon-relay-ng, or another process on the system.

### Increasing the limit

As a workaround, you can increase the limit on your system by modifying `/etc/security/limits.conf` and using `ulimit`; and/or by changing limits in the systemd unit file.
Many webpages describe this process, e.g. [this one](http://woshub.com/too-many-open-files-error-linux/)

### Diagnosing carbon-relay-ng

First obtain the PID of carbon-relay-ng using `pgrep -fl carbon-relay-ng` or `ps aux | grep carbon-relay-ng`.

To see all open FD's of carbon-relay-ng you can then run `sudo ls -l /proc/$pid/fd` or `lsof -p $pid` (or `lsof` for the entire system)
To obtain counts, add `| wc -l` to any of these commands.

You can also get a goroutine dump (stack dump) on `http://localhost:8081/debug/pprof/goroutine?debug=2` (or change the port as needed to match your `http_addr`)

To request support, please run these 3 commands, provide their output and the resulting 2 files.

```
sudo ls -l /proc/$pid/fd > crng-fd.txt
curl 'http://localhost:8081/debug/pprof/goroutine?debug=2' -o crng-goroutine.txt
sudo lsof | wc -l
```
