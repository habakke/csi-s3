package s3

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/golang/glog"
	"github.com/minio/minio-go"
)

const (
	metadataName = ".metadata.json"
	fsPrefix     = "csi-fs"
)

type s3Client struct {
	cfg   *Config
	minio *minio.Client
}

type bucket struct {
	Name          string
	Mounter       string
	FSPath        string
	CapacityBytes int64
}

func newS3Client(cfg *Config) (*s3Client, error) {
	var client = &s3Client{}

	client.cfg = cfg
	u, err := url.Parse(client.cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	ssl := u.Scheme == "https"
	endpoint := u.Hostname()
	if u.Port() != "" {
		endpoint = u.Hostname() + ":" + u.Port()
	}
	minioClient, err := minio.NewWithRegion(endpoint, client.cfg.AccessKeyID, client.cfg.SecretAccessKey, ssl, client.cfg.Region)
	if err != nil {
		return nil, err
	}
	client.minio = minioClient
	return client, nil
}

func newS3ClientFromSecrets(secrets map[string]string) (*s3Client, error) {

	if _, exist := secrets["accessKeyID"]; !exist {
		return nil, errors.New("accessKeyID is not found in Secret")
	}

	if _, exist := secrets["secretAccessKey"]; !exist {
		return nil, errors.New("secretAccessKey is not found in Secret")
	}

	if _, exist := secrets["region"]; !exist {
		return nil, errors.New("region is not found in Secret")
	}

	if _, exist := secrets["endpoint"]; !exist {
		return nil, errors.New("endpoint is not found in Secret")
	}

	return newS3ClientFromMap(secrets)
}

func newS3ClientFromMap(confMap map[string]string) (*s3Client, error) {
	return newS3Client(&Config{
		AccessKeyID:     confMap["accessKeyID"],
		SecretAccessKey: confMap["secretAccessKey"],
		Region:          confMap["region"],
		Endpoint:        confMap["endpoint"],
		// Mounter is set in the volume preferences, not secrets
		Mounter: "",
	})
}

func (client *s3Client) bucketExists(bucketName string) (bool, error) {
	return client.minio.BucketExists(bucketName)
}

func (client *s3Client) createBucket(bucketName string) error {
	return client.minio.MakeBucket(bucketName, client.cfg.Region)
}

func (client *s3Client) createPrefix(bucketName string, prefix string) error {
	_, err := client.minio.PutObject(bucketName, prefix+"/", bytes.NewReader([]byte("")), 0, minio.PutObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (client *s3Client) removeBucket(bucketName string) error {
	if err := client.emptyBucket(bucketName); err != nil {
		return err
	}
	return client.minio.RemoveBucket(bucketName)
}

func (client *s3Client) emptyBucket(bucketName string) error {
	objectsCh := make(chan string)
	var listErr error

	go func() {
		defer close(objectsCh)

		doneCh := make(chan struct{})

		defer close(doneCh)

		for object := range client.minio.ListObjects(bucketName, "", true, doneCh) {
			if object.Err != nil {
				listErr = object.Err
				return
			}
			objectsCh <- object.Key
		}
	}()

	if listErr != nil {
		glog.Error("Error listing objects", listErr)
		return listErr
	}

	select {
	default:
		errorCh := client.minio.RemoveObjects(bucketName, objectsCh)
		for e := range errorCh {
			glog.Errorf("Failed to remove object %s, error: %s", e.ObjectName, e.Err)
		}
		if len(errorCh) != 0 {
			return fmt.Errorf("Failed to remove all objects of bucket %s", bucketName)
		}
	}

	// ensure our prefix is also removed
	return client.minio.RemoveObject(bucketName, fsPrefix)
}

func (client *s3Client) setBucket(bucket *bucket) error {
	b := new(bytes.Buffer)
	_ = json.NewEncoder(b).Encode(bucket)
	opts := minio.PutObjectOptions{ContentType: "application/json"}
	_, err := client.minio.PutObject(bucket.Name, metadataName, b, int64(b.Len()), opts)
	return err
}

func (client *s3Client) getBucket(bucketName string) (*bucket, error) {
	opts := minio.GetObjectOptions{}
	obj, err := client.minio.GetObject(bucketName, metadataName, opts)
	if err != nil {
		return nil, err
	}

	// if .metadata.json is not exist inside bucket, create a default bucket
	// todo: check if there is any side effect for controller server
	objInfo, err := obj.Stat()
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return &bucket{
				Name:          bucketName,
				Mounter:       rcloneMounterType,
				FSPath:        "",
				CapacityBytes: 0,
			}, nil
		}

		return nil, err
	}
	b := make([]byte, objInfo.Size)
	_, err = obj.Read(b)

	if err != nil && err != io.EOF {
		return nil, err
	}
	var meta bucket
	err = json.Unmarshal(b, &meta)
	if err != nil {
		return nil, err
	}

	return &meta, nil
}
