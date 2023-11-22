# secure-schotter

This is currently a spike to test the possibility how to make a secure container with a onetime encrypted writable layer.


```
containerd config default | tee /etc/containerd/config.toml
```

/etc/containerd/config.toml
```
[proxy_plugins]
  [proxy_plugins.custom]
    type = "snapshot"
    address = "/tmp/schotter"
```

```
ctr i pull --snapshotter custom public.ecr.aws/mabels/developers-paradise:ghrunner-latest
ctr run --snapshotter custom public.ecr.aws/mabels/developers-paradise:ghrunner-latest onem7
```

cleanup:
```
losetup  | grep scho | awk '{print $1}'  | xargs losetup -d
umount /var/tmp/schotter.dir/default/118/onem7
cryptsetup plainClose schotter-_var_tmp_schotter.dir_default_116_onem7
```