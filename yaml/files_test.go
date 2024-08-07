package yaml

import (
	"capi-bootstrap/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestConstructFile(t *testing.T) {
	type test struct {
		name       string
		input      types.Values
		escapeFile bool
		localPath  string
		manifest   []byte
		want       InitFile
		wantErr    string
	}
	tests := []test{
		{
			name:      "success",
			input:     types.Values{ClusterName: "test-cluster"},
			localPath: "tmpfile",
			manifest: []byte(`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: "[[[ .ClusterName ]]]-bootstrap"
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: "{{ ds.meta_data.region }}"
  type: g6-standard-6`),
			want: InitFile{Path: "/tmp/manifest.yaml",
				Content: `---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: "test-cluster-bootstrap"
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: "{{ ds.meta_data.region }}"
  type: g6-standard-6`},
		},
		{
			name:       "success escaped file",
			input:      types.Values{ClusterName: "test-cluster"},
			localPath:  "tmpfile",
			escapeFile: true,
			manifest: []byte(`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: "[[[ .ClusterName ]]]-bootstrap"
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: "{{ ds.meta_data.region }}"
  type: g6-standard-6`),
			want: InitFile{Path: "/tmp/manifest.yaml",
				Content: `---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: "test-cluster-bootstrap"
spec:
  image: linode/ubuntu22.04
  instanceID: {{ '{{ ds.meta_data.id }}' }}
  providerID: "linode://{{ '{{ ds.meta_data.id }}' }}"
  region: "{{ '{{ ds.meta_data.region }}' }}"
  type: g6-standard-6`},
		},
		{
			name:      "err invalid file",
			input:     types.Values{ClusterName: "test-cluster"},
			localPath: "wrong-tmpfile",
			wantErr:   "error reading file: open wrong-tmpfile: no such file or directory",
		},
		{
			name:      "err invalid parsed template",
			input:     types.Values{ClusterName: "test-cluster"},
			localPath: "tmpfile",
			manifest: []byte(`---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  name: "[[[ {} .ClusterName ]]]-bootstrap"
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: "{{ ds.meta_data.region }}"
  type: g6-standard-6`),
			wantErr: "failed to parse template tmpfile, template: tmpfile:5: unexpected \"{\" in command",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, err := os.MkdirTemp("", "example")
			assert.NoError(t, err)
			defer os.RemoveAll(dir) // clean up
			file := filepath.Join(dir, "tmpfile")
			err = os.WriteFile(file, tc.manifest, 0666)
			assert.NoError(t, err)
			actualManifest, err := ConstructFile("/tmp/manifest.yaml", tc.localPath, os.DirFS(dir), tc.input, tc.escapeFile)
			if tc.wantErr != "" {
				assert.EqualErrorf(t, err, tc.wantErr, "expected error message: %s", tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.Path, actualManifest.Path)
				assert.Equal(t, tc.want.Content, actualManifest.Content)
			}

		})
	}
}

func TestUpdateCluster(t *testing.T) {
	type test struct {
		name  string
		input []string
		want  []string
	}
	manifests := []string{`---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: test-cluster-md-0
  namespace: default
spec:
  clusterName: test-cluster
  replicas: 3
  selector:
    matchLabels: null
  template:
    spec:
      clusterName: test-cluster`,
		`---
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
`}
	expectedManifests := []string{`---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: test-cluster-md-0
  namespace: default
spec:
  clusterName: test-cluster
  replicas: 3
  selector:
    matchLabels: null
  template:
    spec:
      clusterName: test-cluster`,
		`apiVersion: cluster.x-k8s.io/v1beta1
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
`}

	tests := []test{
		{
			name:  "success",
			input: manifests,
			want:  expectedManifests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := UpdateCluster(manifests)
			assert.NoError(t, err)
			for i, actualFile := range tc.input {
				assert.Equal(t, tc.want[i], actualFile, "expected file: %s", tc.want[i])
			}
		})
	}
}

func TestGetClusterDef(t *testing.T) {
	type test struct {
		name  string
		input []string
		want  *capi.Cluster
	}
	tests := []test{
		{
			name: "success",
			input: []string{`---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: test-cluster-md-0
  namespace: default
spec:
  clusterName: test-cluster
  replicas: 3
  selector:
    matchLabels: null
  template:
    spec:
      clusterName: test-cluster`,
				`---
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
`},
			want: &capi.Cluster{
				ObjectMeta: v1meta.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec: capi.ClusterSpec{
					ClusterNetwork: &capi.ClusterNetwork{
						Pods: &capi.NetworkRanges{CIDRBlocks: []string{"10.192.0.0/10"}},
					},
					ControlPlaneRef: &v1.ObjectReference{
						Name:       "test-cluster-control-plane",
						APIVersion: "cluster.x-k8s.io/v1beta1",
						Kind:       "FakeControlPlane",
					},
					InfrastructureRef: &v1.ObjectReference{
						Name:       "test-cluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
						Kind:       "FakeInfrastructureCluster",
					},
				},
			}},
		{
			name: "no Cluster kind",
			input: []string{`---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: test-cluster-md-0
  namespace: default
spec:
  clusterName: test-cluster
  replicas: 3
  selector:
    matchLabels: null
  template:
    spec:
      clusterName: test-cluster`},
			want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cluster := GetClusterDef(tc.input)
			if tc.want == nil {
				assert.Nil(t, cluster)
			} else {
				assert.Equal(t, cluster.Name, tc.want.Name)
				assert.Equal(t, cluster.Namespace, tc.want.Namespace)
				assert.Equal(t, cluster.Spec.ClusterNetwork, tc.want.Spec.ClusterNetwork)
				assert.Equal(t, cluster.Spec.InfrastructureRef.Kind, tc.want.Spec.InfrastructureRef.Kind)
				assert.Equal(t, cluster.Spec.InfrastructureRef.Name, tc.want.Spec.InfrastructureRef.Name)
				assert.Equal(t, cluster.Spec.ControlPlaneRef.Kind, tc.want.Spec.ControlPlaneRef.Kind)
				assert.Equal(t, cluster.Spec.ControlPlaneRef.Name, tc.want.Spec.ControlPlaneRef.Name)
			}
		})
	}

}
