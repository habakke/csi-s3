# CSI for S3

This is a Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md)) for S3 (or S3 compatible) storage. This can dynamically allocate buckets and mount them via a fuse mount into any container.

## Status

This is still very experimental and should not be used in any production environment. Unexpected data loss could occur depending on what mounter and S3 storage backend is being used.

## Kubernetes installation

### Requirements

* Kubernetes 1.13+ (CSI v1.0.0 compatibility)
* Kubernetes has to allow privileged containers
* Docker daemon must allow shared mounts (systemd flag `MountFlags=shared`)

### Dynamic Provision

#### 1. Create a secret with your S3 credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: kube-system
  name: csi-s3-secret
  # Namespace depends on the configuration in the storageclass.yaml
  namespace: kube-system
stringData:
  accessKeyID: <YOUR_ACCESS_KEY_ID>
  secretAccessKey: <YOUR_SECRET_ACCES_KEY>
  # For AWS set it to "https://s3.<region>.amazonaws.com"
  endpoint: <S3_ENDPOINT_URL>
  # If not on S3, set it to ""
  region: <S3_REGION>
```

The region can be empty if you are using some other S3 compatible storage.

### 2. Deploy the driver for dynamic provision

```bash
cd deploy/kubernetes/
kubectl create -f provisioner.yaml
kubectl create -f attacher.yaml
kubectl create -f csi-s3.yaml
```

### 3. Create the storage class

```bash
kubectl create -f storageclass.yaml
```

### 4. Test the S3 driver for dynamic provision

1. Create a pvc using the new storage class:

```bash
kubectl create -f pvc.yaml
```

2. Check if the PVC has been bound:

```bash
$ kubectl get pvc csi-s3-pvc
NAME         STATUS    VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
csi-s3-pvc   Bound     pvc-c5d4634f-8507-11e8-9f33-0e243832354b   5Gi        RWO            csi-s3         9s
```

3. Create a test pod which mounts your volume:

```bash
kubectl create -f pod.yaml
```

If the pod can start, everything should be working.

4. Test the mount

```bash
$ kubectl exec -ti csi-s3-test-nginx bash
$ mount | grep fuse
s3fs on /var/lib/www/html type fuse.s3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)
$ touch /var/lib/www/html/hello_world
```

If something does not work as expected, check the troubleshooting section below.


### Static Provision


#### 1. Deploy the driver for static provision

```bash
cd deploy/kubernetes/
kubectl create -f provisioner.yaml
kubectl create -f attacher.yaml
kubectl create -f csi-s3.yaml
```


#### 2. Test the S3 driver for static provision

* Create a bucket in your S3 first. 


* Create a secret hold your S3 credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: default
  name: user-s3-secret
stringData:
  accessKeyID: <YOUR_ACCESS_KEY_ID>
  secretAccessKey: <YOUR_SECRET_ACCES_KEY>
  # For AWS set it to "https://s3.<region>.amazonaws.com"
  endpoint: https://s3.eu-central-1.amazonaws.com
  # If not on S3, set it to ""
  region: <S3_REGION>
```

* Create a Persistent Volume represent the existing S3 bucket and specify secret namespace and name.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: new-pv
spec:
  accessModes:
    - ReadWriteOnce
  capacity:
    storage: 5Gi
  csi:
    driver: net.matrise.csi.s3-driver
    volumeAttributes:
      secretNamespace: <SECRET_NAMESPACE_HOLD_YOUR_S3_KEY>
      secretName: <SECRET_NAME_HOLD_YOUR_S3_KEY>
    volumeHandle: <S3_BUCKET_NAME>
```

* Create PVC and check if bound to PV

```bash
$ cd example/static/
$ kubectl apply -f pvc.yaml
$ kubectl apply -f pod.yaml
```

* Test the mount

```bash
$ kubectl exec -ti csi-s3-test-nginx bash
$ mount | grep fuse
s3fs on /var/lib/www/html type fuse.s3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)
$ touch /var/lib/www/html/hello_world
```


## Additional configuration

### Mounter

As S3 is not a real file system there are some limitations to consider here. Depending on what mounter you are using, you will have different levels of POSIX compability. Also depending on what S3 storage backend you are using there are not always [consistency guarantees](https://github.com/gaul/are-we-consistent-yet#observed-consistency).

The driver can be configured to use one of these mounters to mount buckets:

* [rclone](https://rclone.org/commands/rclone_mount)
* [s3fs](https://github.com/s3fs-fuse/s3fs-fuse)
* [goofys](https://github.com/kahing/goofys)
* [s3backer](https://github.com/archiecobbs/s3backer)

The mounter can be set as a parameter in the storage class. You can also create multiple storage classes for each mounter if you like.

All mounters have different strengths and weaknesses depending on your use case. Here are some characteristics which should help you choose a mounter:

#### rclone

* Almost full POSIX compatibility (depends on caching mode)
* Files can be viewed normally with any S3 client

#### s3fs

* Large subset of POSIX
* Files can be viewed normally with any S3 client
* Does not support appends or random writes

#### goofys

* Weak POSIX compatibility
* Performance first
* Files can be viewed normally with any S3 client
* Does not support appends or random writes

#### s3backer (experimental*)

* Represents a block device stored on S3
* Allows to use a real filesystem
* Files are not readable with other S3 clients
* Support appends
* Supports compression before upload (Not yet implemented in this driver)
* Supports encryption before upload (Not yet implemented in this driver)

*s3backer is experimental at this point because volume corruption can occur pretty quickly in case of an unexpected shutdown of a Kubernetes node or CSI pod.
The s3backer binary is not bundled with the normal docker image to keep that as small as possible. Use the `<version>-full` image tag for testing s3backer.

Fore more detailed limitations consult the documentation of the different projects.

## Troubleshooting

### Issues while creating PVC

Check the logs of the provisioner:

```bash
kubectl logs -l app=csi-provisioner-s3 -c csi-s3
```

### Issues creating containers

1. Ensure feature gate `MountPropagation` is not set to `false`
2. Check the logs of the s3-driver:

```bash
kubectl logs -l app=csi-s3 -c csi-s3
```

## Development

This project can be built like any other go application.

```bash
go get -u github.com/ctrox/csi-s3
```

### Build executable

```bash
make build
```

### Tests

Currently the driver is tested by the [CSI Sanity Tester](https://github.com/kubernetes-csi/csi-test/tree/master/pkg/sanity). As end-to-end tests require S3 storage and a mounter like s3fs, this is best done in a docker container. A Dockerfile and the test script are in the `test` directory. The easiest way to run the tests is to just use the make command:

```bash
make test
```


## TODO

* Implement ControllerGetVolume in controllerserver.go (CSI 1.3)
* Update deployment Yaml files (https://github.com/kubernetes-csi/csi-driver-host-path)
* Add support for running tests to GitHub Actions (docker run --rm --privileged -v $(pwd):/app --device /dev/fuse ctrox/csi-s3:test)
* Add support for micro image (https://github.com/ctrox/csi-s3/compare/master...mgrechukh:master)
