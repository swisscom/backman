package s3

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"github.com/minio/minio-go/v6/pkg/encrypt"
	"io"
	"io/ioutil"
	"sort"

	"github.com/minio/minio-go/v6"
	"github.com/minio/sio"
	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/log"
)

func (s *Client) List(folderPath string) ([]minio.ObjectInfo, error) {
	log.Debugf("list S3 object [%s]", folderPath)

	objects := make([]minio.ObjectInfo, 0)
	done := make(chan struct{})
	defer close(done)

	isRecursive := true
	objectCh := s.Client.ListObjectsV2(s.BucketName, folderPath, isRecursive, done)
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, object)
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	return objects, nil
}

func (s *Client) Upload(object string, reader io.Reader, size int64) error {
	return s.UploadWithContext(context.Background(), object, reader, size)
}

func (s *Client) UploadWithContext(ctx context.Context, object string, reader io.Reader, size int64) error {
	log.Debugf("upload S3 object [%s]", object)
	if size <= 0 {
		size = -1
	}

	var err error
	uploadReader := reader
	if len(config.Get().S3.EncryptionKey) != 0 {
		key := getKey(config.Get().S3.EncryptionKey)
		uploadReader, err = sio.EncryptReader(reader, sio.Config{Key: key, CipherSuites: []byte{sio.AES_256_GCM}})
		if err != nil {
			log.Debugf("failed to encrypt reader: %v", err)
			return err
		}
	}

	putOptions := minio.PutObjectOptions{ContentType: "application/gzip"}
	if len(config.Get().S3.ServerSideEncryption) != 0 {
		putOptions.ServerSideEncryption = encrypt.NewSSE()
	}

	n, err := s.Client.PutObjectWithContext(ctx, s.BucketName, object, uploadReader, size, putOptions)
	if err != nil {
		return err
	}
	log.Debugf("uploaded S3 object [%s] of size [%d] bytes", object, n)
	return nil
}

func (s *Client) Stat(object string) (*minio.ObjectInfo, error) {
	log.Debugf("stat S3 object [%s]", object)
	stat, err := s.Client.StatObject(s.BucketName, object, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &stat, nil
}

func (s *Client) Download(object string) (io.Reader, error) {
	return s.DownloadWithContext(context.Background(), object)
}

func (s *Client) DownloadWithContext(ctx context.Context, object string) (io.ReadCloser, error) {
	log.Debugf("download S3 object [%s]", object)
	getOptions := minio.GetObjectOptions{}
	if len(config.Get().S3.ServerSideEncryption) != 0 {
		getOptions.ServerSideEncryption = encrypt.NewSSE()
	}
	reader, err := s.Client.GetObjectWithContext(ctx, s.BucketName, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	if len(config.Get().S3.EncryptionKey) > 0 {
		key := getKey(config.Get().S3.EncryptionKey)
		decrypted, err := sio.DecryptReader(reader, sio.Config{Key: key, CipherSuites: []byte{sio.AES_256_GCM}})
		if err != nil {
			log.Debugf("failed to decrypt reader: %v", err)
			return nil, err
		}
		return ioutil.NopCloser(decrypted), nil
	}
	return reader, nil
}

func (s *Client) Delete(object string) error {
	log.Debugf("delete S3 object [%s]", object)
	if err := s.Client.RemoveObject(s.BucketName, object); err != nil {
		return err
	}
	return nil
}

func getKey(password string) []byte {
	hasher := md5.New()
	hasher.Write([]byte(password))
	return []byte(hex.EncodeToString(hasher.Sum(nil)))
}
