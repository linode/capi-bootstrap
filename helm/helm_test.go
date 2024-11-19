package helm

import (
	"capi-bootstrap/types"
	"testing"

	v1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestHelmProxyToHelmChart(t *testing.T) {
	manifests := []string{`---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: test-cluster-cilium
  namespace: kube-system 
spec:
  chartName: cilium
  clusterSelector:
    matchLabels:
      cni: cilium
  namespace: kube-system
  options:
    timeout: 5m
    wait: true
    waitForJobs: true
  repoURL: https://helm.cilium.io/
  valuesTemplate: |-
    bgpControlPlane:
      enabled: true
    routingMode: native
    kubeProxyReplacement: true
    ipv4NativeRoutingCIDR: 10.0.0.0/8
    tunnelProtocol: ""
    enableIPv4Masquerade: true
    policyAuditMode: true
    hostFirewall:
      enabled: true
    extraConfig:
      allow-localhost: policy
    k8sServiceHost: {{ .InfraCluster.spec.controlPlaneEndpoint.host }}
    k8sServicePort: {{ .InfraCluster.spec.controlPlaneEndpoint.port }}
    extraArgs:
    - --nodeport-addresses=0.0.0.0/0
    ipam:
      mode: kubernetes
    ipv4:
      enabled: true
    ipv6:
      enabled: false
    k8s:
      requireIPv4PodCIDR: true
    hubble:
      relay:
        enabled: true
      ui:
        enabled: true
  version: 1.15.4`, `apiVersion: cluster.x-k8s.io/v1beta1
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
`, `---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: LinodeCluster
metadata:
  name: test-vpc-k3s
  namespace: default
spec:
  credentialsRef:
    name: test-vpc-k3s-credentials
  region: us-mia
  controlPlaneEndpoint:
    host: "api-server.test.com"
    port: 6443
  vpcRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: LinodeVPC
    name: test-vpc-k3s
`}

	helmChartString := `---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: cilium
  namespace: kube-system
spec:
  targetNamespace: kube-system
  version: 1.15.4
  chart: cilium
  repo: https://helm.cilium.io/
  bootstrap: true
  timeout: 5m
  valuesContent: |-
    bgpControlPlane:
      enabled: true
    routingMode: native
    kubeProxyReplacement: true
    ipv4NativeRoutingCIDR: 10.0.0.0/8
    tunnelProtocol: ""
    enableIPv4Masquerade: true
    policyAuditMode: true
    hostFirewall:
      enabled: true
    extraConfig:
      allow-localhost: policy
    k8sServiceHost: api-server.test.com
    k8sServicePort: 6443
    extraArgs:
    - --nodeport-addresses=0.0.0.0/0
    ipam:
      mode: kubernetes
    ipv4:
      enabled: true
    ipv6:
      enabled: false
    k8s:
      requireIPv4PodCIDR: true
    hubble:
      relay:
        enabled: true
      ui:
        enabled: true`

	expectedChart := v1.HelmChart{}
	err := yaml.Unmarshal([]byte(helmChartString), &expectedChart)
	assert.NoError(t, err)
	// expectedHelmChart, err := yaml.Marshal(expectedChart)

	assert.NoError(t, err)
	values := types.Values{ClusterKind: "LinodeCluster", Manifests: manifests}
	helmFiles, err := AddHelmCharts(&values)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(helmFiles))
}
