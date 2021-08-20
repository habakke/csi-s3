module github.com/habakke/csi-s3

go 1.16

require (
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6 // indirect
	github.com/aws/aws-sdk-go v1.35.24 // indirect
	github.com/container-storage-interface/spec v1.3.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/jacobsa/fuse v0.0.0-20180417054321-cd3959611bcb // indirect
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3 // indirect
	github.com/kahing/go-xattr v1.1.1 // indirect
	github.com/kahing/goofys v0.20.0
	github.com/kubernetes-csi/csi-lib-utils v0.6.1 // indirect
	github.com/kubernetes-csi/csi-test v2.0.0+incompatible
	github.com/kubernetes-csi/drivers v1.0.2
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348 // indirect
	github.com/minio/minio-go v0.0.0-20190430232750-10b3660b8f09
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/shirou/gopsutil v0.0.0-20180625081143-4a180b209f5f // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/urfave/cli v1.22.2 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb
	google.golang.org/genproto v0.0.0-20201110150050-8816d57aaa9a // indirect
	google.golang.org/grpc v1.36.0
	gopkg.in/ini.v1 v1.51.0 // indirect
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.21.0
	k8s.io/mount-utils v0.22.1
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace (
	k8s.io/api => k8s.io/api v0.20.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.5
	k8s.io/client-go => k8s.io/client-go v0.20.5
	k8s.io/mount-utils => k8s.io/mount-utils v0.20.5
)
