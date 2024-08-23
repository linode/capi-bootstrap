package k3s

import (
	"context"
	"testing"

	"github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

func TestK3s_GenerateCapiFile(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  *capiYaml.InitFile
	}
	expectedCapiFile := capiYaml.InitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/capi-k3s.yaml",
		Content: `---
apiVersion: v1
kind: Namespace
metadata:
  name: capi-k3s-bootstrap-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: capi-k3s-control-plane-system
---
apiVersion: operator.cluster.x-k8s.io/v1alpha2
kind: BootstrapProvider
metadata:
  name: k3s
  namespace: capi-k3s-bootstrap-system
spec:
  fetchConfig:
    url: https://github.com/k3s-io/cluster-api-k3s/releases/latest/bootstrap-components.yaml
---
apiVersion: operator.cluster.x-k8s.io/v1alpha2
kind: ControlPlaneProvider
metadata:
  name: k3s
  namespace: capi-k3s-control-plane-system
spec:
  fetchConfig:
    url: https://github.com/k3s-io/cluster-api-k3s/releases/latest/control-plane-components.yaml`,
	}
	tests := []test{
		{name: "success", input: types.Values{}, want: ptr.To(expectedCapiFile)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := &ControlPlane{}
			actual, _ := controlPlane.GenerateCapiFile(ctx, &tc.input)
			assert.Equal(t, tc.want.Path, actual.Path, "expected file path: %s", tc.want.Path)
			assert.Equal(t, tc.want.Content, actual.Content, "expected file contents: %s", tc.want.Content)
		})
	}
}

func TestK3s_GenerateAdditionalFiles(t *testing.T) {
	expectedProxyFile := capiYaml.InitFile{
		Path:    etcd.EtcdProxyDaemonsetYamlLocation,
		Content: etcd.EtcdProxyDaemonsetYaml,
	}
	expectedK3sConfig := capiYaml.InitFile{
		Path: "/etc/rancher/k3s/config.yaml",
		Content: `cluster-init: true
disable-cloud-controller: true
kube-apiserver-arg:
- anonymous-auth=true
- tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384
kube-controller-manager-arg:
- cloud-provider=external
kubelet-arg:
- cloud-provider=external
tls-san:
- api-server.test.com
token: test-token
`,
	}
	type test struct {
		name              string
		input             types.Values
		validateBootstrap bool
		want              []capiYaml.InitFile
	}
	tests := []test{
		{
			name: "success defined token",
			input: types.Values{ClusterEndpoint: "api-server.test.com",
				BootstrapToken: "test-token"}, want: []capiYaml.InitFile{expectedK3sConfig, expectedProxyFile},
		},
		{
			name:              "success generate token",
			validateBootstrap: true,
			input:             types.Values{ClusterEndpoint: "api-server.test.com"}, want: []capiYaml.InitFile{expectedK3sConfig, expectedProxyFile},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := &ControlPlane{
				Config: v1beta1.KThreesConfigSpec{
					ServerConfig: v1beta1.KThreesServerConfig{},
					AgentConfig:  v1beta1.KThreesAgentConfig{},
				},
			}
			actual, _ := controlPlane.GenerateAdditionalFiles(ctx, &tc.input)
			assert.NotNil(t, tc.input.BootstrapToken)
			if !tc.validateBootstrap {
				for i, actualFile := range actual {
					assert.Equal(t, tc.want[i].Path, actualFile.Path, "expected file path: %s", tc.want[i].Path)
					assert.Equal(t, tc.want[i].Content, actualFile.Content, "expected file contents: %s", tc.want[i].Content)
				}
			}
		})
	}
}

func TestK3s_PreDeploy(t *testing.T) {
	type test struct {
		name         string
		input        types.Values
		want         types.Values
		wantEndpoint string
		wantErr      string
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
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  kthreesConfigSpec:
    agentConfig:
      nodeName: '{{ ds.meta_data.label }}'
    preK3sCommands:
    - sed -i '/swap/d' /etc/fstab
    - swapoff -a
    - hostnamectl set-hostname '{{ ds.meta_data.label }}' && hostname -F /etc/hostname
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.5+k3s1
`}
	tests := []test{
		{name: "success", input: types.Values{Manifests: manifests, ClusterEndpoint: "api-server.test.com"}, want: types.Values{
			BootstrapManifestDir: "/var/lib/rancher/k3s/server/manifests/",
			K8sVersion:           "v1.29.5+k3s1",
		},
			wantEndpoint: "https://api-server.test.com:6443"},
		{name: "err cp not found", input: types.Values{}, want: types.Values{}, wantErr: "control plane not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := &ControlPlane{
				Config: v1beta1.KThreesConfigSpec{
					ServerConfig: v1beta1.KThreesServerConfig{DisableComponents: []string{"servicelb", "traefik"}},
					AgentConfig:  v1beta1.KThreesAgentConfig{NodeName: "{{ ds.meta_data.label }}"},
				},
			}
			err := controlPlane.PreDeploy(ctx, &tc.input)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.Equalf(t, tc.wantEndpoint, tc.input.Kubeconfig.Clusters[0].Cluster.Server, "expected Server: %v", tc.wantEndpoint)
				assert.Equalf(t, tc.want.K8sVersion, tc.input.K8sVersion, "expected manifest: %v", tc.want.K8sVersion)
				assert.Equalf(t, tc.want.BootstrapManifestDir, tc.input.BootstrapManifestDir, "expected manifest: %v", tc.want.BootstrapManifestDir)
			}
		})
	}
}

func TestK3s_GenerateRunCommand(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  []string
	}
	tests := []test{
		{name: "success", input: types.Values{K8sVersion: "v1.30.0+k3s1"}, want: []string{"curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=\"v1.30.0+k3s1\" sh -s - server"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			actual, _ := controlPlane.GenerateRunCommand(ctx, &tc.input)
			for i, actualCommand := range actual {
				assert.Equal(t, tc.want[i], actualCommand, "expected command: %s", tc.want[i])
			}
		})
	}
}

func TestK3s_GenerateInitScript(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  *capiYaml.InitFile
	}
	expectedFile := capiYaml.InitFile{
		Path: "/tmp/initScript.sh",
		Content: `#!/bin/bash
sed -i "s/127.0.0.1/api-server.test.com/" /etc/rancher/k3s/k3s.yaml
until k3s kubectl get -f /var/lib/rancher/k3s/server/manifests/capi-manifests.yaml; do sleep 10; done
rm /var/lib/rancher/k3s/server/manifests/capi-manifests.yaml
k3s kubectl patch machine test-cluster-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KThreesControlPlane\",\"name\":\"test-cluster-control-plane\",\"uid\":\"$(k3s kubectl get KThreesControlPlane test-cluster-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
k3s kubectl patch cluster test-cluster --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "test-cluster-control-plane"}]'
`,
	}
	tests := []test{
		{name: "success", input: types.Values{ClusterName: "test-cluster", ClusterEndpoint: "api-server.test.com"}, want: &expectedFile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			actual, err := controlPlane.GenerateInitScript(ctx, "/tmp/initScript.sh", &tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.want.Path, actual.Path, "expected file path: %s", tc.want.Path)
			assert.Equal(t, tc.want.Content, actual.Content, "expected file contents: %s", tc.want.Content)
		})
	}
}

func TestK3s_UpdateManifests(t *testing.T) {
	type test struct {
		name  string
		input types.Values
		want  *capiYaml.ParsedManifest
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
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  kthreesConfigSpec:
    files:
    - content: |
        flannel-backend: none
        disable-network-policy: true
      owner: root:root
      path: /etc/rancher/k3s/config.yaml.d/capi-config.yaml
    - content: |-
        apiVersion: helm.cattle.io/v1
        kind: HelmChart
        metadata:
          name: cilium
          namespace: kube-system
        spec:
          targetNamespace: kube-system
          version: 1.15.4
          chart: cilium
      path: /var/lib/rancher/k3s/server/manifests/cilium.yaml
    agentConfig:
      nodeName: '{{ ds.meta_data.label }}'
    preK3sCommands:
    - sed -i '/swap/d' /etc/fstab
    - swapoff -a
    - hostnamectl set-hostname '{{ '{{ ds.meta_data.label }}' }}' && hostname -F /etc/hostname
    postK3sCommands:
    - echo 'success'
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.5+k3s1
`}
	expectedParsedManifest := capiYaml.ParsedManifest{
		ManifestFile: &capiYaml.InitFile{
			Path:    "test",
			Content: "test",
		},
		AdditionalFiles: []capiYaml.InitFile{{Path: "/etc/rancher/k3s/config.yaml.d/capi-config.yaml", Content: "flannel-backend: none\ndisable-network-policy: true\n"},
			{Path: "/var/lib/rancher/k3s/server/manifests/cilium.yaml", Content: "apiVersion: helm.cattle.io/v1\nkind: HelmChart\nmetadata:\n  name: cilium\n  namespace: kube-system\nspec:\n  targetNamespace: kube-system\n  version: 1.15.4\n  chart: cilium"}},
		PreRunCmd:  []string{"sed -i '/swap/d' /etc/fstab", "swapoff -a", "hostnamectl set-hostname '{{ ds.meta_data.label }}' && hostname -F /etc/hostname"},
		PostRunCmd: []string{"echo 'success'"},
	}
	tests := []test{
		{name: "success", input: types.Values{ClusterName: "test-cluster", ClusterEndpoint: "api-server.test.com"}, want: &expectedParsedManifest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			actual, _ := controlPlane.UpdateManifests(ctx, manifests, &tc.input)
			for i, actualFile := range actual.AdditionalFiles {
				assert.Equal(t, tc.want.AdditionalFiles[i].Path, actualFile.Path, "expected file path: %s", tc.want.AdditionalFiles[i].Path)
				assert.Equal(t, tc.want.AdditionalFiles[i].Content, actualFile.Content, "expected file contents: %s", tc.want.AdditionalFiles[i].Content)
			}
			assert.Equal(t, tc.want.PreRunCmd, actual.PreRunCmd, "expected file path: %s", tc.want.PreRunCmd)
			assert.Equal(t, tc.want.PostRunCmd, actual.PostRunCmd, "expected file contents: %s", tc.want.PostRunCmd)
		})
	}
}

func TestK3s_GetControlPlaneCertSecret(t *testing.T) {
	type test struct {
		name  string
		input types.Values
	}
	tests := []test{
		{name: "success", input: types.Values{
			ClusterName: "test-cluster", ClusterEndpoint: "api-server.test.com",
			Manifests: []string{`---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  kthreesConfigSpec:
    agentConfig:
      nodeName: '{{ ds.meta_data.label }}'
    preK3sCommands:
    - sed -i '/swap/d' /etc/fstab
    - swapoff -a
    - hostnamectl set-hostname '{{ ds.meta_data.label }}' && hostname -F /etc/hostname
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.5+k3s1
`}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			err := controlPlane.PreDeploy(ctx, &tc.input)
			assert.NoError(t, err)
			assert.NotNil(t, controlPlane.Certs)
			for _, cert := range controlPlane.Certs {
				assert.NotNil(t, cert.KeyPair)
			}
			secretFile, err := controlPlane.GetControlPlaneCertSecret(ctx, &tc.input)
			assert.NoError(t, err)
			assert.Equal(t, secretFile.Path, "/var/lib/rancher/k3s/server/manifests/cp-secrets.yaml")
			assert.NotNil(t, secretFile.Content)

			// no cert error
			controlPlane.Certs = nil
			_, err = controlPlane.GetControlPlaneCertSecret(ctx, &tc.input)
			assert.True(t, IsErrNoCerts(err))
		})
	}
}

func TestK3s_GetControlPlaneCertFiles(t *testing.T) {
	type test struct {
		name  string
		input types.Values
	}
	tests := []test{
		{name: "success", input: types.Values{
			ClusterName: "test-cluster", ClusterEndpoint: "api-server.test.com",
			Manifests: []string{`---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  kthreesConfigSpec:
    agentConfig:
      nodeName: '{{ ds.meta_data.label }}'
    preK3sCommands:
    - sed -i '/swap/d' /etc/fstab
    - swapoff -a
    - hostnamectl set-hostname '{{ ds.meta_data.label }}' && hostname -F /etc/hostname
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.5+k3s1
`}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			err := controlPlane.PreDeploy(ctx, &tc.input)
			assert.NoError(t, err)
			assert.NotNil(t, controlPlane.Certs)
			for _, cert := range controlPlane.Certs {
				assert.NotNil(t, cert.KeyPair)
			}
			secretFiles, err := controlPlane.GetControlPlaneCertFiles(ctx)
			assert.NoError(t, err)
			assert.Equal(t, len(controlPlane.Certs)*2, len(secretFiles))
			for _, secretFile := range secretFiles {
				assert.NotNil(t, secretFile.Path)
				assert.NotNil(t, secretFile.Content)
			}

			// no cert error
			controlPlane.Certs = nil
			_, err = controlPlane.GetControlPlaneCertFiles(ctx)
			assert.True(t, IsErrNoCerts(err))
		})
	}
}

func TestK3s_GetKubeconfig(t *testing.T) {
	type test struct {
		name  string
		input types.Values
	}
	tests := []test{
		{name: "success", input: types.Values{
			ClusterName: "test-cluster", ClusterEndpoint: "api-server.test.com",
			Manifests: []string{`---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: test-vpc-k3s-control-plane
  namespace: default
spec:
  kthreesConfigSpec:
    agentConfig:
      nodeName: '{{ ds.meta_data.label }}'
    preK3sCommands:
    - sed -i '/swap/d' /etc/fstab
    - swapoff -a
    - hostnamectl set-hostname '{{ ds.meta_data.label }}' && hostname -F /etc/hostname
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.5+k3s1
`}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			controlPlane := ControlPlane{}
			err := controlPlane.PreDeploy(ctx, &tc.input)
			assert.NoError(t, err)
			assert.NotNil(t, controlPlane.Certs)
			for _, cert := range controlPlane.Certs {
				assert.NotNil(t, cert.KeyPair)
			}
			kubeconfig, err := controlPlane.GetKubeconfig(ctx, &tc.input)
			assert.NoError(t, err)
			assert.NotNil(t, kubeconfig)

			// test no cert error
			controlPlane.Certs = nil
			_, err = controlPlane.GetKubeconfig(ctx, &tc.input)
			assert.True(t, IsErrNoCerts(err))
		})
	}
}

func TestNewControlPlane(t *testing.T) {
	k3s := NewControlPlane()
	assert.Equal(t, k3s.Name, "KThreesControlPlane")
}
