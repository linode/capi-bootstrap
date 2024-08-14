package s3

import (
	"bytes"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	k8syaml "sigs.k8s.io/yaml"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func NewBackend() *Backend {
	return &Backend{
		Name: "s3",
	}
}

type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	Options() s3.Options
}

type PresignClient interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type Backend struct {
	Name          string
	Endpoint      string
	Region        string
	BucketName    string
	AccessKey     string
	SecretKey     string
	Client        S3Client      `json:"-"`
	PresignClient PresignClient `json:"-"`
}

func (b *Backend) PreCmd(_ context.Context, _ string) error {

	b.BucketName = os.Getenv("AWS_BUCKET_NAME")
	if b.BucketName == "" {
		return errors.New("AWS_BUCKET_NAME environment variable not set")
	}
	b.AccessKey = os.Getenv("AWS_ACCESS_KEY")
	if b.AccessKey == "" {
		return errors.New("AWS_ACCESS_KEY environment variable not set")
	}
	b.SecretKey = os.Getenv("AWS_SECRET_KEY")
	if b.SecretKey == "" {
		return errors.New("AWS_SECRET_KEY environment variable not set")
	}

	b.Endpoint = os.Getenv("AWS_ENDPOINT")
	b.Region = os.Getenv("AWS_REGION")
	b.Client = s3.New(s3.Options{
		Region:       b.Region,
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(b.AccessKey, b.SecretKey, ""),
	}, func(o *s3.Options) {
		if b.Endpoint != "" {
			o.BaseEndpoint = &b.Endpoint
		}
	})
	b.PresignClient = s3.NewPresignClient(s3.New(b.Client.Options()))

	return nil
}

func (b *Backend) Read(ctx context.Context, clusterName string) (*v1.Config, error) {
	filePath := path.Join("clusters", clusterName, "kubeconfig.yaml")
	remoteFile, err := b.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &b.BucketName,
		Key:    &filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't download object: %v", err)
	}
	if remoteFile == nil {
		return nil, fmt.Errorf("couldn't find object: %s", filePath)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			return
		}
	}(remoteFile.Body)
	state, err := io.ReadAll(remoteFile.Body)
	if err != nil {
		return nil, err
	}
	js, err := k8syaml.YAMLToJSON(state)
	if err != nil {
		return nil, err
	}

	var config v1.Config
	if err = json.Unmarshal(js, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (b *Backend) WriteConfig(ctx context.Context, clusterName string, config *v1.Config) error {
	js, err := json.Marshal(config)
	if err != nil {
		return err
	}

	y, err := k8syaml.JSONToYAML(js)
	if err != nil {
		return err
	}
	filePath := path.Join("clusters", clusterName, "kubeconfig.yaml")
	err = b.uploadFile(ctx, string(y), filePath)
	if err != nil {
		return fmt.Errorf("couldn't upload object: %v", err)
	}
	return nil
}

func (b *Backend) writeFile(ctx context.Context, clusterName string, cloudInitFile capiYaml.InitFile) (string, *capiYaml.InitFile, error) {
	if cloudInitFile.Content == "" {
		return "", nil, errors.New("cloudInitFile content is empty")
	}

	filePath := path.Join("clusters", clusterName, "files", cloudInitFile.Path)
	err := b.uploadFile(ctx, cloudInitFile.Content, filePath)
	if err != nil {
		return "", nil, fmt.Errorf("couldn't upload object: %v", err)
	}

	request, err := b.PresignClient.PresignGetObject(ctx,
		&s3.GetObjectInput{
			Bucket: &b.BucketName,
			Key:    &filePath,
		},
		s3.WithPresignExpires(time.Hour*1), // URL expires in 1 hour
	)
	if err != nil {
		return "nil", nil, fmt.Errorf("couldn't get presigned URL for object: %v", err)
	}
	cloudInitFile.Content = ""
	downloadCmd := fmt.Sprintf("curl -s '%s' | xargs -0 cloud-init query -f > %s", request.URL, cloudInitFile.Path)
	return downloadCmd, &cloudInitFile, nil
}

func (b *Backend) uploadFile(ctx context.Context, fileContent string, filePath string) error {
	br := bytes.NewReader([]byte(fileContent))
	_, err := b.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &b.BucketName,
		Key:    &filePath,
		Body:   br,
	})
	return err
}

func (b *Backend) WriteFiles(ctx context.Context, clusterName string, cloudInitConfig *capiYaml.Config) ([]string, error) {

	downloadCmds := make([]string, len(cloudInitConfig.WriteFiles))
	newFiles := make([]capiYaml.InitFile, len(cloudInitConfig.WriteFiles))
	for i, file := range cloudInitConfig.WriteFiles {
		newCmd, newFile, err := b.writeFile(ctx, clusterName, file)
		if err != nil {
			return nil, err
		}
		downloadCmds[i] = newCmd
		newFiles[i] = *newFile
	}
	cloudInitConfig.WriteFiles = newFiles
	return downloadCmds, nil
}

func (b *Backend) Delete(ctx context.Context, clusterName string) error {
	clusterDir := path.Join("clusters", clusterName)
	objects, err := b.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &b.BucketName,
		Prefix: &clusterDir,
	})
	if err != nil {
		return fmt.Errorf("couldn't list objects: %v", err)
	}
	objectsToDelete := make([]types.ObjectIdentifier, *objects.KeyCount)
	for i, object := range objects.Contents {
		objectsToDelete[i] = types.ObjectIdentifier{Key: object.Key}
	}
	_, err = b.Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: &b.BucketName,
		Delete: &types.Delete{
			Objects: objectsToDelete,
		},
	})
	if err != nil {
		return fmt.Errorf("couldn't delete objects: %v", err)
	}
	return nil
}
