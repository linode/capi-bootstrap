package linode

import (
	mockClient "capi-bootstrap/providers/infrastructure/linode/mock"
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"errors"
	"net"
	"os"
	"testing"

	"github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCAPL_GenerateCapiFile(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  *capiYaml.InitFile
	}
	expectedCapiFile := capiYaml.InitFile{
		Path: "/test-manifests/capi-linode.yaml",
		Content: `---
apiVersion: v1
kind: Namespace
metadata:
  name: capl-system
---
apiVersion: v1
kind: Secret
metadata:
  name: capl-variables
  namespace: capl-system
type: Opaque
stringData:
  LINODE_TOKEN: "test-token"
---
apiVersion: operator.cluster.x-k8s.io/v1alpha2
kind: InfrastructureProvider
metadata:
  name: linode
  namespace: capl-system
spec:
  version: v0.5.0
  fetchConfig:
    url: https://github.com/linode/cluster-api-provider-linode/releases/latest/infrastructure-components.yaml
  configSecret:
    name: capl-variables
`,
	}
	tests := []test{
		{name: "success", input: types.Values{BootstrapManifestDir: "/test-manifests/"}, want: ptr.To(expectedCapiFile)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			Infra := &Infrastructure{
				Token: "test-token",
			}

			actual, _ := Infra.GenerateCapiFile(ctx, &tc.input)
			assert.Equal(t, tc.want.Path, actual.Path, "expected file path: %s", tc.want.Path)
			assert.Equal(t, tc.want.Content, actual.Content, "expected file contents: %s", tc.want.Content)
		})
	}
}

func TestCAPL_GenerateCapiMachine(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  *capiYaml.InitFile
	}
	expectedCapiPivotFile := capiYaml.InitFile{
		Path: "/test-manifests/capi-pivot-machine.yaml",
		Content: `---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "test-cluster"
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: "test-cluster-bootstrap"
  namespace: default
spec:
  bootstrap:
    dataSecretName: linode-test-cluster-crs-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeMachine
    name: "test-cluster-bootstrap"
    namespace: default
  clusterName: "test-cluster"
  providerID: linode://{{ ds.meta_data.id }}
  version: "1.30.0"
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "test-cluster"
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: "test-cluster-bootstrap"
  namespace: default
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: "{{ ds.meta_data.region }}"
  type: g6-standard-6
`,
	}
	tests := []test{
		{name: "success", input: types.Values{ClusterName: "test-cluster", K8sVersion: "1.30.0", BootstrapManifestDir: "/test-manifests/"}, want: ptr.To(expectedCapiPivotFile)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			Infra := Infrastructure{}
			actual, _ := Infra.GenerateCapiMachine(ctx, &tc.input)
			assert.Equal(t, tc.want.Path, actual.Path, "expected file path: %s", tc.want.Path)
			assert.Equal(t, tc.want.Content, actual.Content, "expected file contents: %s", tc.want.Content)
		})
	}
}

func TestCAPL_GenerateAdditionalFiles(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  []capiYaml.InitFile
		infra *Infrastructure
	}
	expectedVPCFile := []capiYaml.InitFile{{
		Path: "/test-manifests/linode-ccm.yaml",
		Content: `---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: ccm-linode
spec:
  targetNamespace: kube-system
  version: v0.4.10
  chart: ccm-linode
  repo: https://linode.github.io/linode-cloud-controller-manager/
  bootstrap: true
  valuesContent: |-
    routeController:
      vpcName: test-cluster
      clusterCIDR: 10.0.0.0/8
      configureCloudRoutes: true
    secretRef:
      name: 'linode-token-region'
    image:
      pullPolicy: IfNotPresent
    secretRef:
      name: 'linode-token-region'
    nodeSelector:
      node-role.kubernetes.io/control-plane: "true"
---
kind: Secret
apiVersion: v1
metadata:
  name: linode-token-region
  namespace: kube-system
stringData:
  apiToken: "test-token"
  region: "{{ ds.meta_data.region }}"
`,
	}}
	expectedVPCLessFile := []capiYaml.InitFile{{
		Path: "/test-manifests/linode-ccm.yaml",
		Content: `---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: ccm-linode
spec:
  targetNamespace: kube-system
  version: v0.4.10
  chart: ccm-linode
  repo: https://linode.github.io/linode-cloud-controller-manager/
  bootstrap: true
  valuesContent: |-
    secretRef:
      name: "linode-token-region"
    image:
      pullPolicy: IfNotPresent
    secretRef:
      name: "linode-token-region"
    nodeSelector:
      node-role.kubernetes.io/control-plane: "true"
---
kind: Secret
apiVersion: v1
metadata:
  name: linode-token-region
  namespace: kube-system
stringData:
  apiToken: "test-token"
  region: "{{ ds.meta_data.region }}"
`,
	}}
	tests := []test{
		{name: "success vpc", infra: &Infrastructure{Token: "test-token", VPC: &v1alpha1.LinodeVPC{}}, input: types.Values{ClusterName: "test-cluster", K8sVersion: "1.30.0", BootstrapManifestDir: "/test-manifests/"}, want: expectedVPCFile},
		{name: "success no vpc", infra: &Infrastructure{Token: "test-token"}, input: types.Values{ClusterName: "test-cluster", K8sVersion: "1.30.0", BootstrapManifestDir: "/test-manifests/"}, want: expectedVPCLessFile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			actualFiles, _ := tc.infra.GenerateAdditionalFiles(ctx, &tc.input)
			for i, actual := range actualFiles {
				assert.Equal(t, tc.want[i].Path, actual.Path, "expected file path: %s", tc.want[i].Path)
				assert.Equal(t, tc.want[i].Content, actual.Content, "expected file contents: %s", tc.want[i].Content)
			}
		})
	}
}

func TestCAPL_PreCmd(t *testing.T) {
	type test struct {
		name  string
		input string
		err   string
	}

	tests := []test{
		{name: "success", input: "test-token", err: ""},
		{name: "err no token", input: "", err: "LINODE_TOKEN env variable is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LINODE_TOKEN", tc.input)
			ctx := context.Background()
			Infra := Infrastructure{}
			actualValues := types.Values{}
			err := Infra.PreCmd(ctx, &actualValues)
			if tc.err != "" {
				assert.EqualErrorf(t, err, tc.err, "expected error message: %s", tc.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, Infra.Client)
			}
		})
	}
}

func TestCAPL_PreDeploy(t *testing.T) {
	type test struct {
		name       string
		input      types.Values
		want       types.Values
		wantErr    string
		mockClient func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient
	}
	manifests := []string{`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: test-cluster-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      interfaces:
      - purpose: public
      region: us-mia
      type: g6-standard-4`,
		`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeVPC
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: test-vpc-k3s
  name: test-cluster
  namespace: default
spec:
  region: us-mia
  subnets:
  - ipv4: 10.0.0.0/8
    label: default
`}

	tests := []test{
		{
			name:  "success",
			input: types.Values{ClusterName: "test-cluster", Manifests: manifests, BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListNodeBalancers(ctx, linodego.NewListOptions(1, `{"tags":"test-cluster"}`)).
					Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().
					CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
						Label:  ptr.To("test-cluster"),
						Region: "us-mia",
						Tags:   []string{"test-cluster"},
					}).
					Return(ptr.To(linodego.NodeBalancer{ID: 123, IPv4: ptr.To("1.2.3.4"), Label: ptr.To("test-cluster")}), nil)
				mock.EXPECT().
					CreateNodeBalancerConfig(ctx, 123, linodego.NodeBalancerConfigCreateOptions{
						Port:      6443,
						Protocol:  "tcp",
						Algorithm: "roundrobin",
						Check:     "connection",
					}).
					Return(ptr.To(linodego.NodeBalancerConfig{ID: 789}), nil)
				return mock
			},
			want: types.Values{
				ClusterEndpoint: "1.2.3.4",
			},
		},
		{
			name:  "err machine not found",
			input: types.Values{ClusterName: "test-cluster", BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				return mock
			},
			wantErr: "machine not found",
		},
		{
			name:  "err list NodeBalancer",
			input: types.Values{ClusterName: "test-cluster", Manifests: manifests, BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListNodeBalancers(ctx, linodego.NewListOptions(1, `{"tags":"test-cluster"}`)).
					Return(nil, errors.New("could not connect to linode"))
				return mock
			},
			wantErr: "unable to list NodeBalancers: could not connect to linode",
		},
		{
			name:  "err existing NodeBalancer",
			input: types.Values{ClusterName: "test-cluster", Manifests: manifests, BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListNodeBalancers(ctx, linodego.NewListOptions(1, `{"tags":"test-cluster"}`)).
					Return([]linodego.NodeBalancer{{ID: 123}}, nil)
				return mock
			},
			wantErr: "node balancer already exists",
		},
		{
			name:  "err create NodeBalancer",
			input: types.Values{ClusterName: "test-cluster", Manifests: manifests, BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListNodeBalancers(ctx, linodego.NewListOptions(1, `{"tags":"test-cluster"}`)).
					Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().
					CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
						Label:  ptr.To("test-cluster"),
						Region: "us-mia",
						Tags:   []string{"test-cluster"},
					}).
					Return(nil, errors.New("could not connect to linode"))
				return mock
			},
			wantErr: "unable to create NodeBalancer: could not connect to linode",
		},
		{
			name:  "err no ipv4",
			input: types.Values{ClusterName: "test-cluster", Manifests: manifests, BootstrapManifestDir: "/test-manifests/"},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListNodeBalancers(ctx, linodego.NewListOptions(1, `{"tags":"test-cluster"}`)).
					Return([]linodego.NodeBalancer{}, nil)
				mock.EXPECT().
					CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
						Label:  ptr.To("test-cluster"),
						Region: "us-mia",
						Tags:   []string{"test-cluster"},
					}).
					Return(ptr.To(linodego.NodeBalancer{ID: 123, Label: ptr.To("test-cluster")}), nil)
				mock.EXPECT().
					CreateNodeBalancerConfig(ctx, 123, linodego.NodeBalancerConfigCreateOptions{
						Port:      6443,
						Protocol:  "tcp",
						Algorithm: "roundrobin",
						Check:     "connection",
					}).
					Return(ptr.To(linodego.NodeBalancerConfig{ID: 789}), nil)
				return mock
			},
			wantErr: "no node IPv4 address on NodeBalancer",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockLinodeClient(ctrl)
			err := os.Setenv("AUTHORIZED_KEYS", "test-key")
			assert.NoError(t, err)
			ctx := context.Background()
			Infra := Infrastructure{
				Client:         tc.mockClient(ctx, t, mock),
				Token:          "test-token",
				AuthorizedKeys: "test-key",
			}
			err = Infra.PreDeploy(ctx, &tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.ClusterEndpoint, tc.input.ClusterEndpoint)
				assert.Equal(t, Infra.AuthorizedKeys, Infra.AuthorizedKeys)
				assert.NotNil(t, Infra.Machine)
				assert.NotNil(t, Infra.NodeBalancer)
				assert.NotNil(t, Infra.NodeBalancerConfig)
				assert.NotNil(t, Infra.VPC)
			} else {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			}

		})
	}
}

func TestCAPL_Deploy(t *testing.T) {
	metadata := []byte("echo 'success'")
	type test struct {
		name       string
		input      types.Values
		want       types.Values
		wantErr    string
		mockClient func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient
	}
	tests := []test{
		{
			name: "success",
			input: types.Values{
				ClusterName:          "test-cluster",
				BootstrapManifestDir: "/test-manifests/",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					CreateVPC(ctx, linodego.VPCCreateOptions{
						Label:       "test-cluster",
						Description: "",
						Region:      "us-mia",
						Subnets: []linodego.VPCSubnetCreateOptions{{
							Label: "pod network",
							IPv4:  "10.0.0.0/8",
						}},
					}).
					Return(ptr.To(linodego.VPC{
						ID:     123,
						Region: "",
						Subnets: []linodego.VPCSubnet{{
							ID: 456,
						}},
					}), nil)
				mock.EXPECT().
					CreateInstance(ctx, gomock.Cond(func(x any) bool {
						createOptions := x.(linodego.InstanceCreateOptions)
						assert.Equal(t, createOptions.Label, "test-cluster-bootstrap")
						assert.Equal(t, createOptions.Region, "us-mia")
						assert.Equal(t, createOptions.Image, "linode/ubuntu")
						assert.Equal(t, createOptions.Type, "nanode")
						assert.Equal(t, createOptions.AuthorizedKeys, []string{"test-key"})
						assert.Equal(t, createOptions.Tags, []string{"test-cluster"})
						assert.Len(t, createOptions.Interfaces, 2)
						assert.Equal(t, createOptions.Interfaces[1].Purpose, linodego.InterfacePurposePublic)
						assert.NotNil(t, createOptions.Metadata)
						return true
					})).
					Return(ptr.To(linodego.Instance{IPv4: []*net.IP{{192, 168, 3, 4}}}), nil)
				mock.EXPECT().
					CreateNodeBalancerNode(ctx, 1234, 5678, linodego.NodeBalancerNodeCreateOptions{
						Address: "192.168.3.4:6443",
						Label:   "test-cluster-bootstrap",
						Weight:  100,
					}).
					Return(ptr.To(linodego.NodeBalancerNode{Label: "test-node"}), nil)
				return mock
			},
			want: types.Values{
				ClusterEndpoint: "1.2.3.4",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockLinodeClient(ctrl)
			err := os.Setenv("AUTHORIZED_KEYS", "test-key")
			assert.NoError(t, err)
			ctx := context.Background()
			Infra := Infrastructure{
				Client: tc.mockClient(ctx, t, mock),
				Machine: &v1alpha1.LinodeMachineTemplate{
					Spec: v1alpha1.LinodeMachineTemplateSpec{
						Template: v1alpha1.LinodeMachineTemplateResource{Spec: v1alpha1.LinodeMachineSpec{
							Image:  "linode/ubuntu",
							Region: "us-mia",
							Type:   "nanode",
						}},
					},
				},
				VPC: &v1alpha1.LinodeVPC{
					ObjectMeta: v1.ObjectMeta{Name: "test-cluster"},
					Spec: v1alpha1.LinodeVPCSpec{
						VPCID:       ptr.To(987),
						Description: "",
						Region:      "us-mia",
						Subnets: []v1alpha1.VPCSubnetCreateOptions{{
							Label: "pod network",
							IPv4:  "10.0.0.0/8",
						}},
						CredentialsRef: nil,
					},
				},
				NodeBalancer: &linodego.NodeBalancer{
					ID: 1234,
				},
				NodeBalancerConfig: &linodego.NodeBalancerConfig{
					ID: 5678,
				},
				Token:          "test-token",
				AuthorizedKeys: "test-key",
			}
			err = Infra.Deploy(ctx, &tc.input, metadata)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			}
		})
	}
}

func TestCAPL_Delete(t *testing.T) {
	type test struct {
		name       string
		input      types.Values
		force      bool
		want       types.Values
		wantErr    string
		mockClient func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient
	}

	tests := []test{
		{
			name: "success",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, x.(*linodego.ListOptions).Filter, `{"tags":"test-cluster"}`)
						return true
					})).
					Return([]linodego.Instance{{ID: 123}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, x.(*linodego.ListOptions).Filter, `{"label":"test-cluster"}`)
						return true
					})).
					Return([]linodego.VPC{{ID: 123}}, nil)
				mock.EXPECT().
					ListNodeBalancers(ctx, gomock.Cond(func(x any) bool {
						assert.Equal(t, x.(*linodego.ListOptions).Filter, `{"tags":"test-cluster"}`)
						return true
					})).
					Return([]linodego.NodeBalancer{{ID: 123, Label: ptr.To("test-cluster")}}, nil)
				mock.EXPECT().
					DeleteInstance(ctx, 123).
					Return(nil)
				mock.EXPECT().
					DeleteNodeBalancer(ctx, 123).
					Return(nil)
				mock.EXPECT().
					DeleteVPC(ctx, 123).
					Return(nil)
				return mock
			},
			force: true,
			want: types.Values{
				ClusterEndpoint: "1.2.3.4",
			},
		},
		{
			name: "err list instances",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return(nil, errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not list instances: could not connect to linode",
		},
		{
			name: "err list VPCs",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return([]linodego.Instance{{ID: 123}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Any()).
					Return(nil, errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not list VPCs: could not connect to linode",
		},
		{
			name: "err list NodeBalancers",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return([]linodego.Instance{{ID: 123}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Any()).
					Return([]linodego.VPC{{ID: 123}}, nil)
				mock.EXPECT().
					ListNodeBalancers(ctx, gomock.Any()).
					Return(nil, errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not list NodeBalancers: could not connect to linode",
		},
		{
			name: "err delete instance",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return([]linodego.Instance{{ID: 123, Label: "test-cluster-bootstrap"}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Any()).
					Return([]linodego.VPC{{ID: 123}}, nil)
				mock.EXPECT().
					ListNodeBalancers(ctx, gomock.Any()).
					Return([]linodego.NodeBalancer{{ID: 123, Label: ptr.To("test-cluster")}}, nil)
				mock.EXPECT().
					DeleteInstance(ctx, gomock.Any()).
					Return(errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not delete Instance test-cluster-bootstrap: could not connect to linode",
		},
		{
			name: "err delete NodeBalancers",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return([]linodego.Instance{{ID: 123}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Any()).
					Return([]linodego.VPC{{ID: 123, Label: "test-cluster"}}, nil)
				mock.EXPECT().
					ListNodeBalancers(ctx, gomock.Any()).
					Return([]linodego.NodeBalancer{{ID: 123, Label: ptr.To("test-cluster")}}, nil)
				mock.EXPECT().
					DeleteInstance(ctx, gomock.Any()).
					Return(nil)
				mock.EXPECT().
					DeleteNodeBalancer(ctx, gomock.Any()).
					Return(errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not delete NodeBalancer test-cluster: could not connect to linode",
		},
		{
			name: "err delete VPC",
			input: types.Values{
				ClusterName: "test-cluster",
			},
			mockClient: func(ctx context.Context, t *testing.T, mock *mockClient.MockLinodeClient) *mockClient.MockLinodeClient {
				mock.EXPECT().
					ListInstances(ctx, gomock.Any()).
					Return([]linodego.Instance{{ID: 123}}, nil)
				mock.EXPECT().
					ListVPCs(ctx, gomock.Any()).
					Return([]linodego.VPC{{ID: 123, Label: "test-cluster"}}, nil)
				mock.EXPECT().
					ListNodeBalancers(ctx, gomock.Any()).
					Return([]linodego.NodeBalancer{{ID: 123, Label: ptr.To("test-cluster")}}, nil)
				mock.EXPECT().
					DeleteInstance(ctx, gomock.Any()).
					Return(nil)
				mock.EXPECT().
					DeleteNodeBalancer(ctx, gomock.Any()).
					Return(nil)
				mock.EXPECT().
					DeleteVPC(ctx, gomock.Any()).
					Return(errors.New("could not connect to linode"))
				return mock
			},
			force:   true,
			wantErr: "could not delete VPC test-cluster: could not connect to linode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mock := mockClient.NewMockLinodeClient(ctrl)
			err := os.Setenv("AUTHORIZED_KEYS", "test-key")
			assert.NoError(t, err)
			ctx := context.Background()
			Infra := Infrastructure{
				Client:         tc.mockClient(ctx, t, mock),
				Token:          "test-token",
				AuthorizedKeys: "test-key",
			}
			err = Infra.Delete(ctx, &tc.input, tc.force)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			}
		})
	}
}
func TestCAPL_UpdateManifests(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  []string
	}
	manifests := []string{`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      interfaces:
      - purpose: public
      region: us-mia
      type: g6-standard-4`,
		`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeCluster
metadata:
  name: test-vpc-k3s
  namespace: default
spec:
  credentialsRef:
    name: test-vpc-k3s-credentials
  region: us-mia
  vpcRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeVPC
    name: test-vpc-k3s
`}
	expectedManifests := []string{`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      interfaces:
      - purpose: public
      region: us-mia
      type: g6-standard-4`,
		`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeCluster
metadata:
  name: test-vpc-k3s
  namespace: default
spec:
  credentialsRef:
    name: test-vpc-k3s-credentials
  region: us-mia
  vpcRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeVPC
    name: test-vpc-k3s
  controlPlaneEndpoint:
    api-server.test.com
    port: 6443
  network
    loadBalancerType: NodeBalancer
    apiserverLoadBalancerPort: 6443
    nodeBalancerID: 1234
    apiserverNodeBalancerConfigID: 5678
`}

	tests := []test{
		{
			name: "success",
			input: types.Values{
				ClusterName:     "test-cluster",
				ClusterEndpoint: "api-server.test.com",
			},
			want: expectedManifests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			infra := &Infrastructure{
				NodeBalancer:       &linodego.NodeBalancer{ID: 1234},
				NodeBalancerConfig: &linodego.NodeBalancerConfig{ID: 5678, Port: 6443},
			}
			err := infra.UpdateManifests(ctx, manifests, &tc.input)
			assert.NoError(t, err)
			for i, actualFile := range tc.input.Manifests {
				assert.Equal(t, tc.want[i], actualFile, "expected file: %s", tc.want[i])
			}
		})
	}
}

func TestCAPL_PostDeploy(t *testing.T) {
	ctx := context.Background()
	infra := Infrastructure{
		NodeBalancer:       &linodego.NodeBalancer{ID: 1234},
		NodeBalancerConfig: &linodego.NodeBalancerConfig{ID: 5678, Port: 6443},
	}
	actualValues := types.Values{}
	err := infra.PostDeploy(ctx, &actualValues)
	assert.NoError(t, err)
}
