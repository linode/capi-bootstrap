package cloudInit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"text/template"
	"time"

	capiYaml "capi-bootstrap/yaml"

	"gopkg.in/yaml.v3"
)

func GenerateCloudInit(values capiYaml.Substitutions, tarWriteFiles bool) ([]byte, error) {
	certManager, err := generateCertManagerManifest(values)
	if err != nil {
		return nil, err
	}
	capiOperator, err := generateCapiOperator(values)
	if err != nil {
		return nil, err
	}

	capiManifests, err := GenerateCapiManifests(values)
	if err != nil {
		return nil, err
	}

	// infra specific
	linodeCCM, err := generateLinodeCCM(values)
	if err != nil {
		return nil, err
	}
	capiLinode, err := generateCapiLinode(values)
	if err != nil {
		return nil, err
	}
	// control plane specific
	k3sProvider, err := generateK3sProvider(values)
	if err != nil {
		return nil, err
	}
	k3sConfig, err := generateK3sConfig(values)
	if err != nil {
		return nil, err
	}
	capiPivotMachine, err := generateCapiPivotMachine(values)
	if err != nil {
		return nil, err
	}

	initScript, err := generateInitScript(values)
	if err != nil {
		return nil, err
	}
	runCmds := []string{fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q sh -", values.K8sVersion),
		"curl -s -L https://github.com/derailed/k9s/releases/download/v0.32.4/k9s_Linux_amd64.tar.gz | tar -xvz -C /usr/local/bin k9s",
		`echo "alias k=\"k3s kubectl\"" >> /root/.bashrc`,
		"echo \"export KUBECONFIG=/etc/rancher/k3s/k3s.yaml\" >> /root/.bashrc",
		"bash /tmp/init-cluster.sh",
	}
	runCmds = append(capiManifests.PreRunCmd, runCmds...)
	runCmds = append(runCmds, capiManifests.PostRunCmd...)
	writeFiles := []capiYaml.InitFile{
		*certManager,
		*capiOperator,
		*k3sProvider,
		*capiLinode,
		*linodeCCM,
		*k3sConfig,
		*capiPivotMachine,
		*capiManifests.ManifestFile,
		*initScript}
	writeFiles = append(writeFiles, capiManifests.AdditionalFiles...)
	if tarWriteFiles {
		fileReader, err := createTar(writeFiles)
		if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(fileReader)
		if err != nil {
			return nil, err
		}

		writeFiles = []capiYaml.InitFile{{
			Path:    "/tmp/cloud-init-files.tgz",
			Content: string(data),
		}}
		runCmds = append([]string{"tar -C / -xvf /tmp/cloud-init-files.tgz", "tar -xf /tmp/cloud-init-files.tgz --to-command='xargs -0 cloud-init query -f > /$TAR_FILENAME'"}, runCmds...)
	}

	cloudConfig := capiYaml.Config{
		WriteFiles: writeFiles,
		RunCmd:     runCmds,
	}

	rawCloudConfig, err := yaml.Marshal(cloudConfig)
	if err != nil {
		return nil, err
	}

	renderedCloudConfig := append([]byte("## template: jinja\n#cloud-config\n\n"), rawCloudConfig...)
	return renderedCloudConfig, nil

}

func generateCertManagerManifest(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/cert-manager.yaml"
	rawContents := `---
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
    installCRDs: true`
	return constructFile(filePath, rawContents, values)
}

func generateCapiOperator(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-operator.yaml"
	rawContents := `---
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
          ClusterTopology: true`
	return constructFile(filePath, rawContents, values)
}

func generateLinodeCCM(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/linode-ccm.yaml"
	rawContents := `---
apiVersion: helm.cattle.io/v1
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
      node-role.kubernetes.io/control-plane: "true"
---
kind: Secret
apiVersion: v1
metadata:
  name: linode-token-region
  namespace: kube-system
stringData:
  apiToken: {{{ .Linode.Token }}}
  region: {{ ds.meta_data.region }}`
	return constructFile(filePath, rawContents, values)
}

func generateK3sProvider(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-k3s.yaml"
	rawContents := `---
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
    url: https://github.com/k3s-io/cluster-api-k3s/releases/latest/control-plane-components.yaml`
	return constructFile(filePath, rawContents, values)
}

func generateCapiLinode(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-linode.yaml"
	rawContents := `---
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
  LINODE_TOKEN: {{{ .Linode.Token }}}
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
    name: capl-variables`
	return constructFile(filePath, rawContents, values)

}

func generateK3sConfig(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/etc/rancher/k3s/config.yaml"
	rawContents := `cluster-init: true
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
node-name: '{{{ .ClusterName }}}-bootstrap'
node-ip:
tls-san:
- {{{ .Linode.NodeBalancerIP }}}
`
	return constructFile(filePath, rawContents, values)
}

func generateCapiPivotMachine(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-pivot-machine.yaml"
	rawContents := `---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: {{{ .ClusterName }}}
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: {{{ .ClusterName }}}-bootstrap
  namespace: default
spec:
  bootstrap:
    dataSecretName: linode-{{{ .ClusterName }}}-crs-0
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeMachine
    name: {{{ .ClusterName }}}-bootstrap
    namespace: default
  clusterName: {{{ .ClusterName }}}
  providerID: linode://{{ ds.meta_data.id }}
  version: {{{ .K8sVersion }}}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachine
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: {{{ .ClusterName }}}
    cluster.x-k8s.io/control-plane: ""
    cluster.x-k8s.io/control-plane-name: ""
  name: {{{ .ClusterName }}}-bootstrap
  namespace: default
spec:
  image: linode/ubuntu22.04
  instanceID: {{ ds.meta_data.id }}
  providerID: "linode://{{ ds.meta_data.id }}"
  region: {{ ds.meta_data.region }}
  type: g6-standard-6`
	return constructFile(filePath, rawContents, values)
}

func GenerateCapiManifests(values capiYaml.Substitutions) (*capiYaml.ParsedManifest, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-pivot-k3s.yaml"
	rawContents := `---
apiVersion: v1
kind: Secret
metadata:
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
  name: {{{ .ClusterName }}}-credentials
  namespace: default
stringData:
  apiToken: {{{ .Linode.Token }}}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  labels:
    cluster: {{{ .ClusterName }}}
  name: {{{ .ClusterName }}}
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.192.0.0/10
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KThreesControlPlane
    name: test-cluster-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeCluster
    name: {{{ .ClusterName }}}
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KThreesControlPlane
metadata:
  name: {{{ .ClusterName }}}-control-plane
  namespace: default
spec:
  infrastructureTemplate:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: LinodeMachineTemplate
    name: {{{ .ClusterName }}}-control-plane
  kthreesConfigSpec:
    files:
      - path: /etc/rancher/k3s/config.yaml.d/capi-config.yaml
        owner: root:root
        content: |
          flannel-backend: none
          disable-network-policy: true
      - path: /var/lib/rancher/k3s/server/manifests/cilium.yaml
        content: |-
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
            valuesContent: |-
              bgpControlPlane:
                enabled: true
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
  name: {{{ .ClusterName }}}
  namespace: default
spec:
  credentialsRef:
    name: {{{ .ClusterName }}}-credentials
  region: us-mia
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LinodeMachineTemplate
metadata:
  name: {{{ .ClusterName }}}-control-plane
  namespace: default
spec:
  template:
    spec:
      image: linode/ubuntu22.04
      region: us-mia
      type: g6-standard-4
      authorizedKeys: [{{{ .Linode.AuthorizedKeys }}}]`
	cloudInitFile, err := constructFile(filePath, rawContents, values)
	if err != nil {
		return nil, err
	}
	var capiManifests *capiYaml.ParsedManifest
	initFileContent, capiManifests, err := capiYaml.UpdateManifest(cloudInitFile.Content, values)
	if err != nil {
		return nil, err
	}
	cloudInitFile.Content = string(initFileContent)
	capiManifests.ManifestFile = cloudInitFile
	return capiManifests, nil

}

func generateInitScript(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/tmp/init-cluster.sh"
	rawContents := `#!/bin/bash
sed -i "s/127.0.0.1/{{{ .Linode.NodeBalancerIP }}}/" /etc/rancher/k3s/k3s.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-kubeconfig --type=cluster.x-k8s.io/secret --from-file=value=/etc/rancher/k3s/k3s.yaml --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-kubeconfig.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-ca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/server-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/server-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-ca.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-cca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/client-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/client-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-cca.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-token --type=cluster.x-k8s.io/secret --from-file=value=/var/lib/rancher/k3s/server/token --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-token.yaml
until k3s kubectl get secret {{{ .ClusterName }}}-ca {{{ .ClusterName }}}-cca {{{ .ClusterName }}}-kubeconfig {{{ .ClusterName }}}-token; do sleep 5; done
k3s kubectl label secret {{{ .ClusterName }}}-kubeconfig {{{ .ClusterName }}}-ca {{{ .ClusterName }}}-cca {{{ .ClusterName }}}-token "cluster.x-k8s.io/cluster-name"="{{{ .ClusterName }}}"
until k3s kubectl get kthreescontrolplane {{{ .ClusterName }}}-control-plane; do sleep 10; done
k3s kubectl patch machine {{{ .ClusterName }}}-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KThreesControlPlane\",\"name\":\"{{{ .ClusterName }}}-control-plane\",\"uid\":\"$(k3s kubectl get KThreesControlPlane {{{ .ClusterName }}}-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
sleep 15
k3s kubectl patch cluster {{{ .ClusterName }}} --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "{{{ .ClusterName }}}-control-plane"}]'`

	return constructFile(filePath, rawContents, values)
}

func templateManifest(rawTemplate string, templateValues capiYaml.Substitutions) ([]byte, error) {
	tmpl := template.New("capi-template")
	tmpl.Delims("{{{", "}}}")
	tmpl, err := tmpl.Parse(rawTemplate)
	if err != nil {
		return nil, err
	}

	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, templateValues)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func constructFile(filePath string, rawContents string, values capiYaml.Substitutions) (*capiYaml.InitFile, error) {

	manifest, err := templateManifest(rawContents, values)
	if err != nil {
		return nil, err
	}
	initFile := capiYaml.InitFile{
		Path:    filePath,
		Content: string(manifest),
	}

	return &initFile, nil
}

func createTar(cloudFiles []capiYaml.InitFile) (io.Reader, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	for _, file := range cloudFiles {
		header := &tar.Header{
			Name:    file.Path[1:],
			Size:    int64(len(file.Content)),
			ModTime: time.Now(),
			Mode:    0644,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, err
		}
		_, err := io.WriteString(tarWriter, file.Content)
		if err != nil {
			return nil, err
		}
	}
	err := tarWriter.Close()
	if err != nil {
		return nil, err
	}
	err = gzipWriter.Close()
	if err != nil {
		return nil, err
	}
	return &buf, nil
}
