package state

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

func TestNewState(t *testing.T) {
	rawExt := `Backend:
  Region: us-mia-1
  Name: s3 
ControlPlane:
  AgentConfig:
    nodeName: node
  Name: KThreesControlPlane
  ServerConfig:
    disableComponents:
    - servicelb
    - traefik
Infrastructure:
  AuthorizedKeys: 
  Machine:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: LinodeMachineTemplate
    metadata:
      creationTimestamp: null
      labels:
        ccm: testcluster1-linode
        cluster: testcluster1
        clusterctl.cluster.x-k8s.io/move: "true"
        crs: testcluster1-crs
        csi: testcluster1-linode
      name: testcluster1-control-plane
      namespace: default
    spec:
      template:
        spec:
          image: linode/ubuntu22.04
          interfaces:
          - purpose: public
          region: us-mia
          type: g6-standard-2
  Name: LinodeCluster
  NodeBalancer:
    client_conn_throttle: 0
    hostname: 192-168-1-1.ip.linodeusercontent.com
    id: 12345
    ipv4: 192.168.1.1
    ipv6: 
    label: testcluster1
    region: us-mia
    tags:
    - testcluster1
    transfer:
      in: null
      out: null
      total: null
  NodeBalancerConfig:
    algorithm: roundrobin
    check: connection
    check_attempts: 3
    check_body: ""
    check_interval: 31
    check_passive: true
    check_path: ""
    check_timeout: 30
    cipher_suite: recommended
    id: 1369120
    nodebalancer_id: 833625
    nodes_status:
      down: 0
      up: 0
    port: 6443
    protocol: tcp
    proxy_protocol: none
    ssl_cert: ""
    ssl_commonname: ""
    ssl_fingerprint: ""
    ssl_key: ""
    stickiness: none
  Token: linodetoken
  VPC:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: LinodeVPC
    metadata:
      creationTimestamp: null
      labels:
        ccm: testcluster1-linode
        cluster: testcluster1
        cluster.x-k8s.io/cluster-name: testcluster1
        clusterctl.cluster.x-k8s.io/move: "true"
        crs: testcluster1-crs
        csi: testcluster1-linode
      name: testcluster1
      namespace: default
    spec:
      description: allow api server traffic
      region: us-mia
      subnets:
      - ipv4: 10.0.0.0/8
        label: default
    status:
      ready: false
Values:
  BootstrapManifestDir: /var/lib/rancher/k3s/server/manifests/
  BootstrapToken: bootstraptoken
  ClusterEndpoint: 192.168.1.1
  ClusterKind: LinodeCluster
  ClusterName: testcluster1
  K8sVersion: v1.29.4+k3s1
  ManifestFile: '-'`

	json, err := yaml.YAMLToJSON([]byte(rawExt))
	assert.NoError(t, err)

	config := &v1.Config{
		Extensions: []v1.NamedExtension{
			{
				Name: ExtensionName,
				Extension: runtime.RawExtension{
					Raw: json,
				},
			},
		},
	}

	state, err := NewState(config)
	assert.NoError(t, err)

	assert.Equal(t, "testcluster1", state.Values.ClusterName)
	assert.Equal(t, config, state.config)
	assert.Equal(t, "*k3s.ControlPlane", reflect.TypeOf(state.ControlPlane).String())
	assert.Equal(t, "*linode.Infrastructure", reflect.TypeOf(state.Infrastructure).String())
	assert.Equal(t, "*s3.Backend", reflect.TypeOf(state.Backend).String())

	// adds state to extension
	c, err := state.ToConfig()
	assert.NoError(t, err)
	assert.Equal(t, config, c)
	assert.Len(t, c.Extensions, 1)

	// removes extra extension
	assert.Len(t, config.Extensions, 1)
	state.config.Extensions = append(state.config.Extensions, v1.NamedExtension{
		Name:      ExtensionName,
		Extension: runtime.RawExtension{},
	}, v1.NamedExtension{
		Name:      "some-other-extension",
		Extension: runtime.RawExtension{},
	})
	assert.Len(t, config.Extensions, 3)

	c, err = state.ToConfig()
	assert.NoError(t, err)
	assert.Equal(t, config, c)
	assert.Len(t, c.Extensions, 2)

	// error parsing the state
	state, err = NewState(&v1.Config{
		Extensions: []v1.NamedExtension{
			{
				Name: ExtensionName,
				Extension: runtime.RawExtension{
					Raw: []byte(`	`), // single tab will parse error
				},
			},
		},
	})
	assert.Nil(t, state)
	assert.Error(t, err)
}
