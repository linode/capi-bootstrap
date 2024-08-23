package cloudinit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mockBackend "capi-bootstrap/providers/backend/mock"
	mockControlplane "capi-bootstrap/providers/controlplane/mock"
	mockInfa "capi-bootstrap/providers/infrastructure/mock"
	"capi-bootstrap/types"
	"capi-bootstrap/yaml"
)

func workingMock(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
	mock.EXPECT().
		UpdateManifests(ctx, gomock.Any(), gomock.Any()).
		Return(nil)
	mock.EXPECT().
		GenerateAdditionalFiles(ctx, gomock.Any()).
		Return([]yaml.InitFile{{Path: "/tmp/infra-additional.txt", Content: "additional text"}}, nil)
	mock.EXPECT().
		GenerateCapiFile(ctx, gomock.Any()).
		Return(&yaml.InitFile{Path: "/tmp/infraCapi.yaml"}, nil)
	mock.EXPECT().
		GenerateCapiMachine(ctx, gomock.Any()).
		Return(&yaml.InitFile{Path: "/tmp/capiMachine.yaml"}, nil)
	return mock
}

func TestGenerateCloudInit(t *testing.T) {
	expectedManifest := `## template: jinja
#cloud-config

write_files:
    - path: /tmp/cert-manager.yaml
      content: |
        ---
        apiVersion: helm.cattle.io/v1
        kind: HelmChart
        metadata:
          name: cert-manager
          namespace: kube-system
        spec:
          repo: https://charts.jetstack.io
          chart: cert-manager
          targetNamespace: cert-manager
          createNamespace: true
          bootstrap: true
          valuesContent: |-
            installCRDs: true
    - path: /tmp/capi-operator.yaml
      content: |
        ---
        apiVersion: v1
        kind: Namespace
        metadata:
          name: capi-operator-system
        ---
        apiVersion: helm.cattle.io/v1
        kind: HelmChart
        metadata:
          name: capi-operator
          namespace: capi-operator-system
        spec:
          repo: https://kubernetes-sigs.github.io/cluster-api-operator
          chart: cluster-api-operator
          targetNamespace: capi-operator-system
          createNamespace: true
          bootstrap: true
          valuesContent: |-
            core: cluster-api
            addon: helm
            manager:
              featureGates:
                core:
                  ClusterResourceSet: true
                  ClusterTopology: true
    - path: /tmp/cpCapi.yaml
    - path: /tmp/infraCapi.yaml
    - path: /tmp/capiMachine.yaml
    - path: /tmp/capi-manifests.yaml
      content: |
        ---
        apiVersion: cluster.x-k8s.io/v1beta1
        kind: Cluster
        metadata:
          creationTimestamp: null
          name: test-cluster
          namespace: default
        spec:
          clusterNetwork:
            pods:
              cidrBlocks:
              - 10.192.0.0/10
          controlPlaneEndpoint:
            host: ""
            port: 0
          controlPlaneRef:
            apiVersion: controlplane.cluster.x-k8s.io/v1beta1
            kind: FakeControlPlane
            name: fake-control-plane
          infrastructureRef:
            apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
            kind: FakeInfrastructureCluster
            name: test-cluster
        status:
          controlPlaneReady: false
          infrastructureReady: false
    - path: /tmp/test.cert
    - path: /tmp/kubeconfig
    - path: /tmp/init-cluster.sh
    - path: /tmp/infra-additional.txt
      content: additional text
    - path: /tmp/cp-additional.txt
    - path: /tmp/certs.yaml
runcmd:
    - curl install-manifests
    - curl install-k8s.com
    - curl -s -L https://github.com/derailed/k9s/releases/download/v0.32.4/k9s_Linux_amd64.tar.gz | tar -xvz -C /usr/local/bin k9s
    - echo "alias k=\"k3s kubectl\"" >> /root/.bashrc
    - echo "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml" >> /root/.bashrc
    - bash /tmp/init-cluster.sh
`
	expectedTarManfest := `## template: jinja
#cloud-config

write_files:
    - path: /tmp/cloud-init-files.tgz
      content: !!binary |`
	manifestInput := `---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.192.0.0/10
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: FakeControlPlane
    name: test-cluster-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: FakeInfrastructureCluster
    name: test-cluster

`
	type test struct {
		name                   string
		manifest               string
		value                  types.Values
		want                   string
		wantErr                string
		mockInfraClient        func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider
		mockControlPlaneClient func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider
		mocBackendClient       func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider
	}

	tests := []test{
		{
			name:            "success",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, nil)
				mock.EXPECT().
					GetControlPlaneCertSecret(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/test.cert"}, nil)
				mock.EXPECT().
					GetKubeconfig(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/kubeconfig"}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				mock.EXPECT().
					WriteFiles(ctx, gomock.Any(), gomock.Any()).
					Return([]string{"curl install-manifests"}, nil)
				return mock
			},
			manifest: manifestInput,
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
			},
			want: expectedManifest,
		},
		{
			name:            "success tar files",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, nil)
				mock.EXPECT().
					GetControlPlaneCertSecret(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/test.cert"}, nil)
				mock.EXPECT().
					GetKubeconfig(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/kubeconfig"}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				mock.EXPECT().
					WriteFiles(ctx, gomock.Any(), gomock.Any()).
					Return([]string{"curl install-manifests"}, nil)
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			want:     expectedTarManfest,
		},
		{
			name: "err updateManifests",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(errors.New("infra error"))
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name: "err infra additional files",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return(nil, errors.New("infra error"))
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name: "err infra capi files",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return(nil, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/infraCapi.yaml"}, errors.New("infra error"))
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name: "err infra capi machine",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/infra-additional.txt", Content: "additional text"}}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/infraCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateCapiMachine(ctx, gomock.Any()).
					Return(nil, errors.New("infra error"))
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate capi file",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate additionalFiles",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate init script",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate run command",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate cert files",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate controlplaneSecret",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, nil)
				mock.EXPECT().
					GetControlPlaneCertSecret(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/test.cert"}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err controlplane generate kubeconfig",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, nil)
				mock.EXPECT().
					GetControlPlaneCertSecret(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/test.cert"}, nil)
				mock.EXPECT().
					GetKubeconfig(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/kubeconfig"}, errors.New("infra error"))
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
		{
			name:            "err backend write files",
			mockInfraClient: workingMock,
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{}, nil)
				mock.EXPECT().
					GenerateCapiFile(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/cpCapi.yaml"}, nil)
				mock.EXPECT().
					GenerateAdditionalFiles(ctx, gomock.Any()).
					Return([]yaml.InitFile{{Path: "/tmp/cp-additional.txt"}}, nil)
				mock.EXPECT().
					GenerateInitScript(ctx, "/tmp/init-cluster.sh", gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/init-cluster.sh"}, nil)
				mock.EXPECT().
					GenerateRunCommand(ctx, gomock.Any()).
					Return([]string{"curl install-k8s.com"}, nil)
				mock.EXPECT().
					GetControlPlaneCertFiles(ctx).
					Return([]yaml.InitFile{{Path: "/tmp/certs.yaml"}}, nil)
				mock.EXPECT().
					GetControlPlaneCertSecret(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/test.cert"}, nil)
				mock.EXPECT().
					GetKubeconfig(ctx, gomock.Any()).
					Return(&yaml.InitFile{Path: "/tmp/kubeconfig"}, nil)
				return mock
			},
			mocBackendClient: func(ctx context.Context, t *testing.T, mock *mockBackend.MockProvider) *mockBackend.MockProvider {
				mock.EXPECT().
					WriteFiles(ctx, gomock.Any(), gomock.Any()).
					Return([]string{"curl install-manifests"}, errors.New("infra error"))
				return mock
			},
			value: types.Values{
				ManifestFile:         "tmpfile",
				BootstrapManifestDir: "/tmp/",
				TarWriteFiles:        true,
			},
			manifest: manifestInput,
			wantErr:  "infra error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir, err := os.MkdirTemp("", "example")
			assert.NoError(t, err)
			defer os.RemoveAll(dir) // clean up

			tc.value.ManifestFS = os.DirFS(dir)
			file := filepath.Join(dir, "tmpfile")
			err = os.WriteFile(file, []byte(tc.manifest), 0666)
			assert.NoError(t, err)
			ctrl := gomock.NewController(t)
			infraMock := mockInfa.NewMockProvider(ctrl)
			controlPlaneMock := mockControlplane.NewMockProvider(ctrl)
			backendMock := mockBackend.NewMockProvider(ctrl)
			infra := tc.mockInfraClient(ctx, t, infraMock)
			controlPlane := tc.mockControlPlaneClient(ctx, t, controlPlaneMock)
			backend := tc.mocBackendClient(ctx, t, backendMock)
			manifest, err := GenerateCloudInit(ctx, &tc.value, infra, controlPlane, backend)

			if tc.wantErr == "" {
				assert.NoError(t, err)
				if tc.value.TarWriteFiles {
					assert.True(t, strings.HasPrefix(string(manifest), tc.want))
				} else {
					assert.Equal(t, tc.want, string(manifest))
				}
			} else {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			}
		})
	}
}
func TestGenerateCapiManifests(t *testing.T) {
	type test struct {
		name                   string
		manifest               string
		wantErr                string
		mockInfraClient        func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider
		mockControlPlaneClient func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider
	}

	tests := []test{
		{
			name: "success",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(&yaml.ParsedManifest{
						ManifestFile:    nil,
						AdditionalFiles: nil,
						PreRunCmd:       []string{"echo 'hello'"},
						PostRunCmd:      nil,
					}, nil)
				return mock
			},
			manifest: `---
test1: false
---
test2: true
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: test-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.192.0.0/10
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: FakeControlPlane
    name: test-cluster-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: FakeInfrastructureCluster
    name: test-cluster
`,
		},
		{
			name: "err construct file",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				return mock
			},
			manifest: "[[[ {} .test ]]]",
			wantErr:  "failed to parse template tmpfile, template: tmpfile:1: unexpected \"{\" in command",
		},
		{
			name: "err update cluster",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				return mock
			},
			manifest: `--`,
			wantErr:  "error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type v1beta1.Cluster",
		},
		{
			name: "err update infra manifests",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(errors.New("failed to update manifests"))
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				return mock
			},
			wantErr: "failed to update manifests",
		},
		{
			name: "err update controlplane manifests",
			mockInfraClient: func(ctx context.Context, t *testing.T, mock *mockInfa.MockProvider) *mockInfa.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
				return mock
			},
			mockControlPlaneClient: func(ctx context.Context, t *testing.T, mock *mockControlplane.MockProvider) *mockControlplane.MockProvider {
				mock.EXPECT().
					UpdateManifests(ctx, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("failed to update manifests"))
				return mock
			},
			wantErr: "failed to update manifests",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dir, err := os.MkdirTemp("", "example")
			assert.NoError(t, err)
			defer os.RemoveAll(dir) // clean up
			file := filepath.Join(dir, "tmpfile")
			err = os.WriteFile(file, []byte(tc.manifest), 0666)
			assert.NoError(t, err)
			ctrl := gomock.NewController(t)
			infraMock := mockInfa.NewMockProvider(ctrl)
			controlPlaneMock := mockControlplane.NewMockProvider(ctrl)
			infra := tc.mockInfraClient(ctx, t, infraMock)
			controlPlane := tc.mockControlPlaneClient(ctx, t, controlPlaneMock)
			values := &types.Values{
				ManifestFile:         "tmpfile",
				ManifestFS:           os.DirFS(dir),
				BootstrapManifestDir: "/tmp/",
			}
			manifest, err := GenerateCapiManifests(ctx, values, infra, controlPlane, false)

			if tc.wantErr == "" {
				assert.NoError(t, err)
				assert.Equal(t, "echo 'hello'", manifest.PreRunCmd[0])
				assert.NotNil(t, manifest)
			} else {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			}
		})
	}
}
