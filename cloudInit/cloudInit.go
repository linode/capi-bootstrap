package cloudInit

import (
	"bytes"
	"gopkg.in/yaml.v3"
	"text/template"
)

type cloudInitFile struct {
	Path        string `yaml:"path"`
	Content     string `yaml:"content"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
}
type Config struct {
	WriteFiles []cloudInitFile `yaml:"write_files"`
	RunCmd     []string        `yaml:"runcmd"`
}

func GenerateCloudInit(clusterName string) (string, error) {
	certManager, err := generateCertManagerManifest()
	if err != nil {
		return "", err
	}
	capiOperator, err := generateCapiOperator()
	if err != nil {
		return "", err
	}
	linodeCCM, err := generateLinodeCCM()
	if err != nil {
		return "", err
	}
	linodeToken, err := generateLinodeToken()
	if err != nil {
		return "", err
	}
	k3sPovider, err := generateK3sProvider()
	if err != nil {
		return "", err
	}
	capiLinode, err := generateCapiLinode()
	if err != nil {
		return "", err
	}
	ciliumConfig, err := generateCiliumConfig()
	if err != nil {
		return "", err
	}
	k3sConfig, err := generateK3sConfig()
	if err != nil {
		return "", err
	}
	capiPivotMachine, err := generateCapiPivotMachine(clusterName)
	if err != nil {
		return "", err
	}
	capiManifest, err := generateCapiManifests(clusterName)
	if err != nil {
		return "", err
	}
	initScript, err := generateInitScript(clusterName)
	if err != nil {
		return "", err
	}
	cloudConfig := Config{
		WriteFiles: []cloudInitFile{*certManager, *capiOperator, *linodeCCM, *linodeToken, *k3sPovider, *capiLinode, *ciliumConfig, *k3sConfig, *capiPivotMachine, *capiManifest, *initScript},
		RunCmd: []string{`echo "node-ip: $(hostname -I | grep -oE 192\.168\.[0-9]+\.[0-9]+)" >> /etc/rancher/k3s/config.yaml`,
			"curl -sfL https://get.k3s.io | sh -",
			"curl -s -L https://github.com/derailed/k9s/releases/download/v0.32.4/k9s_Linux_amd64.tar.gz | tar -xvz -C /usr/local/bin k9s",
			`echo "alias k=\"k3s kubectl\"" >> /root/.bashrc`,
			"echo \"export KUBECONFIG=/etc/rancher/k3s/k3s.yaml\" >> /root/.bashrc",
			"bash /tmp/init-cluster.sh",
		},
	}

	rawCloudConfig, err := yaml.Marshal(cloudConfig)
	if err != nil {
		return "", err
	}
	renderedCloudConfig := "## template: jinja\n#cloud-config\n\n" + string(rawCloudConfig)
	return renderedCloudConfig, nil

}

func generateCertManagerManifest() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/cert-manager.yaml",
		Content: `---
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
  valuesContent: |-
    installCRDs: true`,
	}, nil
}

func generateCapiOperator() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/capi-operator.yaml",
		Content: `---
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
  valuesContent: |-
    core: cluster-api
    addon: helm
    manager:
      featureGates:
        core:
          ClusterResourceSet: true
          ClusterTopology: true`,
	}, nil
}

func generateLinodeCCM() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/linode-ccm.yaml",
		Content: `apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: ccm-linode
spec:
  targetNamespace: kube-system
  version: v0.4.4
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
      node-role.kubernetes.io/control-plane: "true"`,
	}, nil
}

func generateLinodeToken() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/linode-token-region.yaml",
		Content: `kind: Secret
apiVersion: v1
metadata:
  name: linode-token-region
  namespace: kube-system
stringData:
  apiToken: ${LINODE_TOKEN}
  region: {{ ds.meta_data.region }}`,
	}, nil
}

func generateK3sProvider() (*cloudInitFile, error) {
	return &cloudInitFile{
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
	}, nil
}

func generateCapiLinode() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/capi-linode.yaml",
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
  LINODE_TOKEN: ${LINODE_TOKEN}
---
apiVersion: operator.cluster.x-k8s.io/v1alpha2
kind: InfrastructureProvider
metadata:
  name: linode
  namespace: capl-system
spec:
  version: v0.3.0
  fetchConfig:
    url: https://github.com/linode/cluster-api-provider-linode/releases/latest/infrastructure-components.yaml
  configSecret:
    name: capl-variables`,
	}, nil
}

func generateCiliumConfig() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path: "/var/lib/rancher/k3s/server/manifests/cilium-config.yaml",
		Content: `apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: cilium
  namespace: kube-system
spec:
  targetNamespace: kube-system
  version: 1.15.0
  chart: cilium
  repo: https://helm.cilium.io/
  bootstrap: true
  valuesContent: |-
    bgpControlPlane:
      enabled: true
    ipam:
      mode: kubernetes
    k8s:
      requireIPv4PodCIDR: true
    hubble:
      relay:
        enabled: true
      ui:
        enabled: true`,
	}, nil
}

func generateK3sConfig() (*cloudInitFile, error) {
	return &cloudInitFile{
		Path:        "/etc/rancher/k3s/config.yaml",
		Owner:       "root:root",
		Permissions: "0640",
		Content: `cluster-init: true
flannel-backend: none
disable-network-policy: true
disable-cloud-controller: true
disable:
- servicelb
- traefik
disable-cloud-controller: true
kube-apiserver-arg:
- anonymous-auth=true
- tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384
kube-controller-manager-arg:
- cloud-provider=external
kubelet-arg:
- cloud-provider=external
node-name: '{{ ds.meta_data.label }}'
node-ip:
tls-san:
- ${NB_IP}
`,
	}, nil
}

func generateCapiPivotMachine(clusterName string) (*cloudInitFile, error) {
	rawContents := `---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: {{{ .cluster_name }}}
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: {{{ .cluster_name }}}-bootstrap
  namespace: default
spec:
  bootstrap:
    dataSecretName: linode-{{{ .cluster_name }}}-crs-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeMachine
    name: {{{ .cluster_name }}}-bootstrap
    namespace: default
  clusterName: {{{ .cluster_name }}}
  providerID: linode://{{ ds.meta_data.id }}
  version: v1.29.4+k3s1
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: {{{ .cluster_name }}}
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: {{{ .cluster_name }}}-bootstrap
  namespace: default
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: {{ ds.meta_data.region }}
  type: g6-standard-6`
	values := map[string]string{"cluster_name": clusterName}
	manifest, err := templateManifest(rawContents, values)
	if err != nil {
		return nil, err
	}
	initFile := cloudInitFile{
		Path:    "/var/lib/rancher/k3s/server/manifests/capi-pivot-machine.yaml",
		Content: manifest,
	}
	return &initFile, nil
}

func generateCapiManifests(clusterName string) (*cloudInitFile, error) {
	rawContents := `apiVersion: v1
kind: Secret
metadata:
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
  name: {{{ .cluster_name }}}-credentials
  namespace: default
stringData:
  apiToken: ${LINODE_TOKEN}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  labels:
    cluster: {{{ .cluster_name }}}
  name: {{{ .cluster_name }}}
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.192.0.0/10
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KThreesControlPlane
    name: fake-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeCluster
    name: {{{ .cluster_name }}}
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: {{{ .cluster_name }}}-control-plane
  namespace: default
spec:
  infrastructureTemplate:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeMachineTemplate
    name: {{{ .cluster_name }}}-control-plane
  kthreesConfigSpec:
    files:
      - path: /etc/rancher/k3s/config.yaml.d/capi-config.yaml
        owner: root:root
        content: |
          flannel-backend: none
          disable-network-policy: true
    agentConfig:
      nodeName: "{{ '{{ ds.meta_data.label }}' }}"
    preK3sCommands:
      - |
        echo "node-ip: $(hostname -I | grep -oE 192\.168\.[0-9]+\.[0-9]+)" >> /etc/rancher/k3s/config.yaml.d/capi-config.yaml
      - sed -i '/swap/d' /etc/fstab
      - swapoff -a
      - hostnamectl set-hostname "{{ '{{ ds.meta_data.label }}' }}" && hostname -F /etc/hostname
    serverConfig:
      disableComponents:
      - servicelb
      - traefik
  replicas: 3
  version: v1.29.4+k3s1
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeCluster
metadata:
  name: {{{ .cluster_name }}}
  namespace: default
spec:
  credentialsRef:
    name: {{{ .cluster_name }}}-credentials
  region: {{ ds.meta_data.region }}
  controlPlaneEndpoint:
    host: ${NB_IP}
    port: ${NB_PORT}
  network:
    loadBalancerType: NodeBalancer
    nodeBalancerConfigID: ${NB_CONFIG_ID}
    nodeBalancerID: ${NB_ID}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: {{{ .cluster_name }}}-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      region: {{ ds.meta_data.region }}
      type: g6-standard-4
      authorizedKeys: ${AUTHORIZED_KEYS}`
	values := map[string]string{"cluster_name": clusterName}
	manifest, err := templateManifest(rawContents, values)
	if err != nil {
		return nil, err
	}
	initFile := cloudInitFile{
		Path:    "/var/lib/rancher/k3s/server/manifests/capi-pivot-k3s.yaml",
		Content: manifest,
	}
	return &initFile, nil
}

func generateInitScript(clusterName string) (*cloudInitFile, error) {
	rawContents := `#!/bin/bash
sed -i "s/127.0.0.1/${NB_IP}/" /etc/rancher/k3s/k3s.yaml
k3s kubectl create secret generic {{{ .cluster_name }}}-kubeconfig --type=cluster.x-k8s.io/secret --from-file=value=/etc/rancher/k3s/k3s.yaml --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-kubeconfig.yaml
k3s kubectl create secret generic {{{ .cluster_name }}}-ca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/server-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/server-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-ca.yaml
k3s kubectl create secret generic {{{ .cluster_name }}}-cca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/client-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/client-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-cca.yaml
k3s kubectl create secret generic {{{ .cluster_name }}}-token --type=cluster.x-k8s.io/secret --from-file=value=/var/lib/rancher/k3s/server/token --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-token.yaml
until k3s kubectl get secret {{{ .cluster_name }}}-ca {{{ .cluster_name }}}-cca {{{ .cluster_name }}}-kubeconfig {{{ .cluster_name }}}-token; do sleep 5; done
k3s kubectl label secret {{{ .cluster_name }}}-kubeconfig {{{ .cluster_name }}}-ca {{{ .cluster_name }}}-cca {{{ .cluster_name }}}-token "cluster.x-k8s.io/cluster-name"="{{{ .cluster_name }}}"
until k3s kubectl get kthreescontrolplane {{{ .cluster_name }}}-control-plane; do sleep 10; done
k3s kubectl patch machine {{{ .cluster_name }}}-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KThreesControlPlane\",\"name\":\"{{{ .cluster_name }}}-control-plane\",\"uid\":\"$(k3s kubectl get KThreesControlPlane {{{ .cluster_name }}}-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
sleep 15
k3s kubectl patch cluster {{{ .cluster_name }}} --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "{{{ .cluster_name }}}-control-plane"}]'`
	values := map[string]string{"cluster_name": clusterName}
	manifest, err := templateManifest(rawContents, values)
	if err != nil {
		return nil, err
	}
	initFile := cloudInitFile{
		Path:    "/tmp/init-cluster.sh",
		Content: manifest,
	}
	return &initFile, nil
}

func templateManifest(rawTemplate string, templateValues map[string]string) (string, error) {
	tmpl := template.New("capi-template")
	tmpl.Delims("{{{", "}}}")
	tmpl, err := tmpl.Parse(rawTemplate)
	if err != nil {
		return "", err
	}

	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, templateValues)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
