/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package s3

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/mount-utils"

	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	kclient *kubernetes.Clientset
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	targetPath := req.GetTargetPath()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Target path missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	notMnt, err := checkMount(targetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	deviceID := ""
	if req.GetPublishContext() != nil {
		deviceID = req.GetPublishContext()[deviceID]
	}

	// TODO: Implement readOnly & mountFlags
	readOnly := req.GetReadonly()
	// TODO: check if attrib is correct with context.
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	glog.V(4).Infof("target %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, deviceID, readOnly, volumeID, attrib, mountFlags)

	var s3 *s3Client
	// if volume attribute contain secret name & namespace, retrieve S3 credentials from this secret.
	// Otherwise, use S3 credentials stored in secret from request.
	if checkS3SecretExist(attrib) {
		s3, err = newS3ClientFromSecrets(ns.getExistS3BucketCredentials(ctx, attrib["secretNamespace"], attrib["secretName"]))
	} else {
		s3, err = newS3ClientFromSecrets(req.GetSecrets())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}

	exists, err := s3.bucketExists(volumeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket %s exists: %v", volumeID, err)
	}

	if !exists {
		return nil, fmt.Errorf("bucket %s not exist", volumeID)
	}

	b, err := s3.getBucket(volumeID)
	if err != nil {
		return nil, err
	}

	mounter, err := newMounter(b, s3.cfg)
	if err != nil {
		return nil, err
	}
	if err := mounter.Mount(stagingTargetPath, targetPath); err != nil {
		return nil, err
	}

	glog.V(4).Infof("s3: bucket %s successfuly mounted to %s", b.Name, targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if err := fuseUnmount(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	glog.V(4).Infof("s3: bucket %s has been unmounted.", volumeID)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided")
	}

	notMnt, err := checkMount(stagingTargetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !notMnt {
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// if volume attribute contain secret name & namespace, retrieve S3 credentials from this secret.
	// Otherwise, use S3 credentials stored in secret from request.
	var s3 *s3Client
	attrib := req.GetVolumeContext()
	if checkS3SecretExist(attrib) {
		s3, err = newS3ClientFromSecrets(ns.getExistS3BucketCredentials(ctx, attrib["secretNamespace"], attrib["secretName"]))
	} else {
		s3, err = newS3ClientFromSecrets(req.GetSecrets())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}

	exists, err := s3.bucketExists(volumeID)

	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket %s exists: %v", volumeID, err)
	}

	if !exists {
		return nil, fmt.Errorf("bucket %s not exist", volumeID)
	}

	b, err := s3.getBucket(volumeID)
	if err != nil {
		return nil, err
	}
	mounter, err := newMounter(b, s3.cfg)
	if err != nil {
		return nil, err
	}
	if err := mounter.Stage(stagingTargetPath); err != nil {
		return nil, err
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	stagingTargetPath := req.GetStagingTargetPath()

	// Check arguments
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// currently there is a single NodeServer capability according to the spec
	nscap := &csi.NodeServiceCapability{
		Type: &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			},
		},
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			nscap,
		},
	}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

func (ns *nodeServer) getExistS3BucketCredentials(ctx context.Context, namespace, name string) map[string]string {

	result := make(map[string]string)
	secret, err := ns.kclient.CoreV1().Secrets(namespace).Get(ctx, name, v1.GetOptions{})

	if err != nil {
		glog.V(4).Infof("%s", err.Error())
		return result
	}

	for k, v := range secret.Data {
		result[k] = string(v)
	}

	return result
}

func checkMount(targetPath string) (bool, error) {
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return false, err
			}
			notMnt = true
		} else {
			return false, err
		}
	}
	return notMnt, nil
}

func checkS3SecretExist(attr map[string]string) bool {

	if _, exist := attr["secretNamespace"]; !exist {
		return false
	}

	if _, exist := attr["secretName"]; !exist {
		return false
	}

	return true
}
