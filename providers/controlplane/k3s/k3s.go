package k3s

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	capK3s "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta1"
	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"github.com/k3s-io/cluster-api-k3s/pkg/kubeconfig"
	secrets "github.com/k3s-io/cluster-api-k3s/pkg/secret"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

type ControlPlane struct {
	Name   string
	Config v1beta1.KThreesConfigSpec
	Certs  secrets.Certificates
}

var ErrNoCerts = errors.New("missing control plane certs")

func IsErrNoCerts(err error) bool {
	return err == ErrNoCerts
}

func NewControlPlane() *ControlPlane {
	return &ControlPlane{
		Name: "KThreesControlPlane",
	}
}

func (p *ControlPlane) GenerateCapiFile(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-k3s.yaml"
	return capiYaml.ConstructFile(filePath, "files/capi-k3s.yaml", files, values, false)
}

func (p *ControlPlane) GenerateAdditionalFiles(_ context.Context, values *types.Values) ([]capiYaml.InitFile, error) {
	configFile, err := p.generateK3sConfig(values)
	if err != nil {
		return nil, err
	}
	manifestFile, err := generateK3sManifests()
	if err != nil {
		return nil, err
	}
	return []capiYaml.InitFile{
		*configFile, *manifestFile,
	}, nil
}

func (p *ControlPlane) PreDeploy(ctx context.Context, values *types.Values) error {
	// parse the controlPlane from the manifests
	controlPlaneSpec := GetControlPlaneDef(values.Manifests)
	if controlPlaneSpec == nil {
		return errors.New("control plane not found")
	}

	p.Config = v1beta1.KThreesConfigSpec{
		ServerConfig: controlPlaneSpec.Spec.KThreesConfigSpec.ServerConfig,
		AgentConfig:  controlPlaneSpec.Spec.KThreesConfigSpec.AgentConfig,
	}

	// set the k8s version as parsed from the ControlPlane
	values.K8sVersion = controlPlaneSpec.Spec.Version
	klog.Infof("k8s version : %s", controlPlaneSpec.Spec.Version)

	// generate certificates
	p.Certs = secrets.NewCertificatesForInitialControlPlane(&p.Config)
	for _, cert := range p.Certs {
		err := cert.Generate()
		if err != nil {
			return err
		}
	}
	// generate kubeconfig
	var clientCACert, serverCACert *x509.Certificate
	var clientCAKey crypto.Signer
	var err error
	for _, cert := range p.Certs {
		switch cert.Purpose {
		case secrets.ClusterCA:
			serverCACert, err = certs.DecodeCertPEM(cert.KeyPair.Cert)
			if err != nil {
				return errors.Join(errors.New("failed to decode server CA certificate"), err)
			}
		case secrets.ClientClusterCA:
			clientCACert, err = certs.DecodeCertPEM(cert.KeyPair.Cert)
			if err != nil {
				return errors.Join(errors.New("failed to decode client cluster CA certificate"), err)
			}
			clientCAKey, err = certs.DecodePrivateKeyPEM(cert.KeyPair.Key)
			if err != nil {
				return errors.Join(errors.New("failed to decode client CA private key"), err)
			}
		}
	}
	newKubeconfig, err := kubeconfig.New(values.ClusterName, fmt.Sprintf("https://%s", net.JoinHostPort(values.ClusterEndpoint, "6443")), clientCACert, clientCAKey, serverCACert)
	if err != nil {
		return errors.Join(errors.New("failed to generate kubeconfig"), err)
	}
	values.Kubeconfig = &clientv1.Config{}
	err = clientv1.Convert_api_Config_To_v1_Config(newKubeconfig, values.Kubeconfig, nil)
	if err != nil {
		return errors.Join(errors.New("failed to convert kubeconfig to v1"), err)
	}
	// set the BootstrapManifestDir
	values.BootstrapManifestDir = "/var/lib/rancher/k3s/server/manifests/"

	return nil
}

func (p *ControlPlane) GenerateRunCommand(_ context.Context, values *types.Values) ([]string, error) {
	return []string{fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q sh -s - server", values.K8sVersion)}, nil
}

func (p *ControlPlane) GenerateInitScript(_ context.Context, initScriptPath string, values *types.Values) (*capiYaml.InitFile, error) {
	return capiYaml.ConstructFile(initScriptPath, "files/init-cluster.sh", files, values, false)
}

func GetControlPlaneDef(manifests []string) *capK3s.KThreesControlPlane {
	var cp capK3s.KThreesControlPlane
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &cp)
		if cp.Kind == "KThreesControlPlane" {
			return &cp
		}
	}
	return nil
}

func (p *ControlPlane) generateK3sConfig(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/etc/rancher/k3s/config.yaml"
	if values.BootstrapToken == "" {
		values.BootstrapToken = uuid.NewString()
	}
	config := k3s.GenerateInitControlPlaneConfig(values.ClusterEndpoint, values.BootstrapToken, p.Config.ServerConfig, p.Config.AgentConfig)
	configYaml, err := capiYaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return &capiYaml.InitFile{
		Path:    filePath,
		Content: string(configYaml),
	}, nil
}

func generateK3sManifests() (*capiYaml.InitFile, error) {
	return &capiYaml.InitFile{
		Path:    etcd.EtcdProxyDaemonsetYamlLocation,
		Content: etcd.EtcdProxyDaemonsetYaml,
	}, nil
}

func (p *ControlPlane) UpdateManifests(_ context.Context, manifests []string, values *types.Values) (*capiYaml.ParsedManifest, error) {
	var controlPlane capK3s.KThreesControlPlane
	var controlPlaneManifests capiYaml.ParsedManifest
	for _, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &controlPlane)
		if err != nil {
			return nil, err
		}
		if controlPlane.Kind == "KThreesControlPlane" {
			for _, file := range controlPlane.Spec.KThreesConfigSpec.Files {
				newFile := capiYaml.InitFile{
					Path:        file.Path,
					Content:     file.Content,
					Owner:       file.Owner,
					Permissions: file.Permissions,
					Encoding:    string(file.Encoding),
				}
				controlPlaneManifests.AdditionalFiles = append(controlPlaneManifests.AdditionalFiles, newFile)
			}
			for _, cmd := range controlPlane.Spec.KThreesConfigSpec.PreK3sCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PreRunCmd = append(controlPlaneManifests.PreRunCmd, parsedCommand)
			}
			for _, cmd := range controlPlane.Spec.KThreesConfigSpec.PostK3sCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PostRunCmd = append(controlPlaneManifests.PostRunCmd, parsedCommand)
			}
		}

	}
	return &controlPlaneManifests, nil
}

func (p *ControlPlane) GetControlPlaneCertSecret(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	if p.Certs == nil {
		return nil, ErrNoCerts
	}
	certSecrets := v1.SecretList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
	}
	for _, cert := range p.Certs {
		certSecret := cert.AsSecret(client.ObjectKey{
			Namespace: values.Namespace,
			Name:      values.ClusterName,
		}, metav1.OwnerReference{})
		certSecret.TypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}
		certSecret.OwnerReferences = nil
		certSecrets.Items = append(certSecrets.Items, *certSecret)
	}
	tokenSecret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secrets.Name(values.ClusterName, "token"),
			Namespace: values.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: values.ClusterName,
			},
		},
		Data: map[string][]byte{
			"value": []byte(values.BootstrapToken),
		},
		Type: clusterv1.ClusterSecretType,
	}
	certSecrets.Items = append(certSecrets.Items, tokenSecret)
	secretString, err := yaml.Marshal(certSecrets)
	if err != nil {
		return nil, err
	}
	secretFile := capiYaml.InitFile{
		Path:    path.Join(values.BootstrapManifestDir + "cp-secrets.yaml"),
		Content: string(secretString),
	}

	return &secretFile, nil

}

func (p *ControlPlane) GetControlPlaneCertFiles(ctx context.Context) ([]capiYaml.InitFile, error) {
	if p.Certs == nil {
		return nil, ErrNoCerts
	}
	k3sFiles := p.Certs.AsFiles()
	yamlFiles := make([]capiYaml.InitFile, len(k3sFiles))
	for i, file := range k3sFiles {
		yamlFiles[i] = capiYaml.InitFile{
			Path:        file.Path,
			Content:     file.Content,
			Owner:       file.Owner,
			Permissions: file.Permissions,
			Encoding:    string(file.Encoding),
		}
	}
	return yamlFiles, nil
}

func (p *ControlPlane) GetKubeconfig(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	if p.Certs == nil {
		return nil, ErrNoCerts
	}

	kubeconfigBytes, err := yaml.Marshal(values.Kubeconfig)
	if err != nil {
		return nil, err
	}
	kubeconfigSecret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secrets.Name(values.ClusterName, secrets.Kubeconfig),
			Namespace: values.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: values.ClusterName,
			},
		},
		Data: map[string][]byte{
			secrets.KubeconfigDataName: kubeconfigBytes,
		},
	}
	secretBytes, err := yaml.Marshal(kubeconfigSecret)
	if err != nil {
		return nil, err
	}

	kubeconfigFile := capiYaml.InitFile{
		Path:    path.Join(values.BootstrapManifestDir + "kubeconfig-secret.yaml"),
		Content: string(secretBytes),
	}

	return &kubeconfigFile, nil
}
