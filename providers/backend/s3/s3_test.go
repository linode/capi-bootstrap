package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"

	mockClient "capi-bootstrap/providers/backend/s3/mock"
	capiYaml "capi-bootstrap/yaml"
)

func TestS3_PreCmd(t *testing.T) {
	type test struct {
		name        string
		accessKey   string
		secretKey   string
		bucketName  string
		endpoint    string
		region      string
		clusterName string
		err         string
	}

	tests := []test{
		{
			name:        "success",
			accessKey:   "access_key",
			secretKey:   "secret_key",
			bucketName:  "test-bucket",
			endpoint:    "test-endpoint.com",
			region:      "us-east",
			clusterName: "test-cluster",
			err:         "",
		},
		{name: "err no bucket name", err: "AWS_BUCKET_NAME environment variable not set"},
		{name: "err no access key", bucketName: "test", err: "AWS_ACCESS_KEY environment variable not set"},
		{name: "err no secret key", bucketName: "test", accessKey: "test-key", err: "AWS_SECRET_KEY environment variable not set"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_BUCKET_NAME", tc.bucketName)
			t.Setenv("AWS_ENDPOINT", tc.endpoint)
			t.Setenv("AWS_REGION", tc.region)
			t.Setenv("AWS_ACCESS_KEY", tc.accessKey)
			t.Setenv("AWS_SECRET_KEY", tc.secretKey)
			ctx := context.Background()
			testBackend := Backend{}
			err := testBackend.PreCmd(ctx, tc.clusterName)
			if tc.err != "" {
				assert.EqualErrorf(t, err, tc.err, "expected error message: %s", tc.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testBackend.AccessKey, tc.accessKey)
				assert.Equal(t, testBackend.SecretKey, tc.secretKey)
				assert.Equal(t, testBackend.Region, tc.region)
				assert.Equal(t, testBackend.BucketName, tc.bucketName)
				assert.NotNil(t, testBackend.Client)
				assert.NotNil(t, testBackend.PresignClient)
			}
		})
	}
}

func TestS3_Read(t *testing.T) {
	type test struct {
		name        string
		bucketName  string
		clusterName string
		want        v1.Config
		wantErr     string
		mockClient  func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client
	}
	tests := []test{
		{
			name:        "success",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					GetObject(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, "test-bucket", *x.(*s3.GetObjectInput).Bucket)
						assert.Equal(t, "clusters/test-cluster/kubeconfig.yaml", *x.(*s3.GetObjectInput).Key)
						return true
					})).
					Return(&s3.GetObjectOutput{
						Body: io.NopCloser(bytes.NewReader([]byte(`---
clusters:
- cluster:
   server: https://123.456.789:6443
  name: test-cluster
`))),
					}, nil)
				return mock
			},
			want: v1.Config{
				Clusters: []v1.NamedCluster{{
					Name: "test-cluster",
					Cluster: v1.Cluster{
						Server: "https://123.456.789:6443",
					},
				}},
			},
		},
		{
			name:        "err download failure",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					GetObject(ctx, gomock.Any()).
					Return(nil, errors.New("s3 failure"))
				return mock
			},
			wantErr: "couldn't download object: s3 failure",
		},
		{
			name:        "err empty config",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				err := smithy.GenericAPIError{Code: "NoSuchKey"}
				mock.EXPECT().
					GetObject(ctx, gomock.Any()).
					Return(nil, &err)
				return mock
			},
			wantErr: "couldn't find object: clusters/test-cluster/kubeconfig.yaml",
		},
		{
			name:        "err invalid yaml",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					GetObject(ctx, gomock.Any()).
					Return(&s3.GetObjectOutput{
						Body: io.NopCloser(bytes.NewReader([]byte("}{"))),
					}, nil)
				return mock
			},
			wantErr: "yaml: did not find expected node content",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockS3Client(ctrl)
			ctx := context.Background()
			testBackend := NewBackend()
			testBackend.BucketName = tc.bucketName
			testBackend.Client = tc.mockClient(ctx, t, mock)
			actualConfig, err := testBackend.Read(ctx, tc.clusterName)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.Clusters[0].Name, actualConfig.Clusters[0].Name)
				assert.Equal(t, tc.want.Clusters[0].Cluster.Server, actualConfig.Clusters[0].Cluster.Server)
			}
		})
	}
}

func TestS3_WriteConfig(t *testing.T) {
	type test struct {
		name        string
		bucketName  string
		clusterName string
		wantErr     string
		mockClient  func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client
	}
	tests := []test{
		{
			name:        "success",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					PutObject(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.PutObjectInput).Bucket)
						assert.Equal(t, `clusters/test-cluster/kubeconfig.yaml`, *x.(*s3.PutObjectInput).Key)
						state, err := io.ReadAll(x.(*s3.PutObjectInput).Body)
						assert.NoError(t, err)
						assert.Equal(t, `clusters: null
contexts: null
current-context: testContext
preferences: {}
users: null
`, string(state))
						return true
					})).
					Return(nil, nil)
				return mock
			},
			wantErr: "",
		},
		{
			name:        "err upload failure",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					PutObject(ctx, gomock.Any()).
					Return(nil, errors.New("s3 failure"))
				return mock
			},
			wantErr: "couldn't upload object: s3 failure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockS3Client(ctrl)
			ctx := context.Background()
			testBackend := NewBackend()
			testBackend.BucketName = tc.bucketName
			testBackend.Client = tc.mockClient(ctx, t, mock)
			err := testBackend.WriteConfig(ctx, tc.clusterName, &v1.Config{CurrentContext: "testContext"})
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestS3_WriteFiles(t *testing.T) {
	type test struct {
		name              string
		bucketName        string
		clusterName       string
		wantErr           string
		mockClient        func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client
		mockPresignClient func(ctx context.Context, t *testing.T, mock *mockClient.MockPresignClient) *mockClient.MockPresignClient
	}

	tests := []test{
		{
			name:        "success",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					PutObject(ctx, gomock.Any()).
					Return(&s3.PutObjectOutput{}, nil).Times(2)
				return mock
			},
			mockPresignClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockPresignClient) *mockClient.MockPresignClient {
				mock.EXPECT().
					PresignGetObject(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.GetObjectInput).Bucket)
						assert.Equal(t, `clusters/test-cluster/files/tmp/test1.yaml`, *x.(*s3.GetObjectInput).Key)
						return true
					}), gomock.Any()).
					Return(&v4.PresignedHTTPRequest{
						URL: "signed.test.com/tmp/test1.yaml",
					}, nil).Times(1)
				mock.EXPECT().
					PresignGetObject(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.GetObjectInput).Bucket)
						assert.Equal(t, `clusters/test-cluster/files/tmp/test2.yaml`, *x.(*s3.GetObjectInput).Key)
						return true
					}), gomock.Any()).
					Return(&v4.PresignedHTTPRequest{
						URL: "signed.test.com/tmp/test2.yaml",
					}, nil)
				return mock
			},
		},
		{
			name:        "err upload object",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					PutObject(ctx, gomock.Any()).
					Return(nil, errors.New("s3 failure"))
				return mock
			},
			mockPresignClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockPresignClient) *mockClient.MockPresignClient {
				return mock
			},
			wantErr: "couldn't upload object: s3 failure",
		},
		{
			name:        "err presign url",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					PutObject(ctx, gomock.Any()).
					Return(&s3.PutObjectOutput{}, nil)
				return mock
			},
			mockPresignClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockPresignClient) *mockClient.MockPresignClient {
				mock.EXPECT().
					PresignGetObject(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("s3 failure"))
				return mock
			},
			wantErr: "couldn't get presigned URL for object: s3 failure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockS3Client(ctrl)
			mockPreSign := mockClient.NewMockPresignClient(ctrl)
			ctx := context.Background()
			testBackend := NewBackend()
			testBackend.BucketName = tc.bucketName
			testBackend.Client = tc.mockClient(ctx, t, mock)
			testBackend.PresignClient = tc.mockPresignClient(ctx, t, mockPreSign)
			cloudInitFile := capiYaml.Config{
				WriteFiles: []capiYaml.InitFile{{
					Path:    "/tmp/test1.yaml",
					Content: "This is test file 1",
				},
					{
						Path:    "/tmp/test2.yaml",
						Content: "This is test file 2",
					}},
				RunCmd: []string{"echo hello"},
			}
			newCmds, err := testBackend.WriteFiles(ctx, tc.clusterName, &cloudInitFile)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, []string{"curl -s 'signed.test.com/tmp/test1.yaml' | xargs -0 cloud-init query -f > /tmp/test1.yaml", "curl -s 'signed.test.com/tmp/test2.yaml' | xargs -0 cloud-init query -f > /tmp/test2.yaml"}, newCmds)
				for _, file := range cloudInitFile.WriteFiles {
					assert.Empty(t, file.Content)
				}
			}
		})
	}
}
func TestS3_Delete(t *testing.T) {
	type test struct {
		name        string
		bucketName  string
		clusterName string
		want        v1.Config
		wantErr     string
		mockClient  func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client
	}
	tests := []test{
		{
			name:        "success",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.ListObjectsV2Input).Bucket)
						assert.Equal(t, `clusters/test-cluster`, *x.(*s3.ListObjectsV2Input).Prefix)
						return true
					})).
					Return(&s3.ListObjectsV2Output{
						KeyCount: ptr.To(int32(2)),
						Contents: []s3Types.Object{{
							Key: ptr.To("clusters/test-cluster/file1.yaml"),
						},
							{
								Key: ptr.To("clusters/test-cluster/file2.yaml"),
							}},
					}, nil)
				mock.EXPECT().
					DeleteObjects(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.DeleteObjectsInput).Bucket)
						assert.Equal(t, s3Types.Delete{
							Objects: []s3Types.ObjectIdentifier{{Key: ptr.To("clusters/test-cluster/file1.yaml")},
								{Key: ptr.To("clusters/test-cluster/file2.yaml")}},
						}, *x.(*s3.DeleteObjectsInput).Delete)
						return true
					})).
					Return(&s3.DeleteObjectsOutput{}, nil)
				return mock
			},
			want: v1.Config{
				Clusters: []v1.NamedCluster{{
					Name: "test-cluster",
					Cluster: v1.Cluster{
						Server: "https://123.456.789:6443",
					},
				}},
			},
		},
		{
			name:        "err list objects",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Any()).
					Return(&s3.ListObjectsV2Output{
						KeyCount: ptr.To(int32(2)),
						Contents: []s3Types.Object{{
							Key: ptr.To("clusters/test-cluster/file1.yaml"),
						},
							{
								Key: ptr.To("clusters/test-cluster/file2.yaml"),
							}},
					}, nil)
				mock.EXPECT().
					DeleteObjects(ctx, gomock.Any()).
					Return(nil, errors.New("s3 error"))
				return mock
			},
			wantErr: "couldn't delete objects: s3 error",
		},
		{
			name:        "err list objects",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Any()).
					Return(nil, errors.New("s3 error"))
				return mock
			},
			wantErr: "couldn't list objects: s3 error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockS3Client(ctrl)
			ctx := context.Background()
			testBackend := NewBackend()
			testBackend.BucketName = tc.bucketName
			testBackend.Client = tc.mockClient(ctx, t, mock)
			err := testBackend.Delete(ctx, tc.clusterName)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestS3_List(t *testing.T) {
	type test struct {
		name        string
		bucketName  string
		clusterName string
		want        v1.Config
		wantErr     string
		mockClient  func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client
	}
	tests := []test{
		{
			name:        "success",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, `test-bucket`, *x.(*s3.ListObjectsV2Input).Bucket)
						assert.Equal(t, `clusters/`, *x.(*s3.ListObjectsV2Input).Prefix)
						assert.Equal(t, `/`, *x.(*s3.ListObjectsV2Input).Delimiter)
						return true
					})).
					Return(&s3.ListObjectsV2Output{
						CommonPrefixes: []s3Types.CommonPrefix{{Prefix: ptr.To("clusters/test-cluster/")}},
						KeyCount:       ptr.To(int32(2)),
						Contents: []s3Types.Object{{
							Key: ptr.To("clusters/test-cluster/file1.yaml"),
						},
							{
								Key: ptr.To("clusters/test-cluster/file2.yaml"),
							}},
					}, nil)
				mock.EXPECT().
					GetObject(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, "test-bucket", *x.(*s3.GetObjectInput).Bucket)
						assert.Equal(t, "clusters/test-cluster/kubeconfig.yaml", *x.(*s3.GetObjectInput).Key)
						return true
					})).
					Return(&s3.GetObjectOutput{
						Body: io.NopCloser(bytes.NewReader([]byte(`---
clusters:
- cluster:
   server: https://123.456.789:6443
  name: test-cluster
`))),
					}, nil)
				return mock
			},
			want: v1.Config{
				Clusters: []v1.NamedCluster{{
					Name: "test-cluster",
					Cluster: v1.Cluster{
						Server: "https://123.456.789:6443",
					},
				}},
			},
		},
		{
			name:        "err get configs",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Any()).
					Return(&s3.ListObjectsV2Output{
						CommonPrefixes: []s3Types.CommonPrefix{{Prefix: ptr.To("clusters/test-cluster/")}},
						KeyCount:       ptr.To(int32(2)),
						Contents: []s3Types.Object{{
							Key: ptr.To("clusters/test-cluster/file1.yaml"),
						},
							{
								Key: ptr.To("clusters/test-cluster/file2.yaml"),
							}},
					}, nil)
				mock.EXPECT().
					GetObject(ctx, gomock.Any()).
					Return(nil, errors.New("s3 error"))
				return mock
			},
			wantErr: "couldn't read cluster config: couldn't download object: s3 error",
		},
		{
			name:        "err list clusters",
			bucketName:  "test-bucket",
			clusterName: "test-cluster",
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockS3Client) *mockClient.MockS3Client {
				mock.EXPECT().
					ListObjectsV2(ctx, gomock.Any()).
					Return(nil, errors.New("s3 error"))
				return mock
			},
			wantErr: "couldn't list clusters: s3 error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockS3Client(ctrl)
			ctx := context.Background()
			testBackend := NewBackend()
			testBackend.BucketName = tc.bucketName
			testBackend.Client = tc.mockClient(ctx, t, mock)
			clusters, err := testBackend.ListClusters(ctx)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, clusters["test-cluster"])
				assert.Equal(t, tc.want.Clusters, clusters["test-cluster"].Clusters)
			}
		})
	}
}
