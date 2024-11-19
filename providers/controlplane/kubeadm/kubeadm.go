package kubeadm

import (
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/google/uuid"

	"sigs.k8s.io/yaml"

	clientv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/kubeconfig"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	kubeadmtypes "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	secrets "sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoCerts = errors.New("missing control plane certs")

type ControlPlane struct {
	Name          string
	ClusterConfig bootstrapv1.ClusterConfiguration
	InitConfig    bootstrapv1.InitConfiguration
	Certs         secrets.Certificates
}

func NewControlPlane() *ControlPlane {
	return &ControlPlane{
		Name: "KubeadmControlPlane",
	}
}

func (p *ControlPlane) GenerateCapiFile(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/kubeadm/manifests/capi-kubeadm.yaml"
	return capiYaml.ConstructFile(filePath, "files/capi-kubeadm.yaml", files, values, false)
}

func (p *ControlPlane) GenerateInitScript(_ context.Context, initScriptPath string, values *types.Values) (*capiYaml.InitFile, error) {
	return capiYaml.ConstructFile(initScriptPath, "files/init-cluster.sh", files, values, false)
}

func (p *ControlPlane) GenerateRunCommand(_ context.Context, _ *types.Values) ([]string, error) {
	return []string{"kubeadm init --config /run/kubeadm/kubeadm.yaml"}, nil
}

func (p *ControlPlane) GenerateAdditionalFiles(_ context.Context, values *types.Values) ([]capiYaml.InitFile, error) {
	configFile, err := p.generateKubeadmConfig(values)
	if err != nil {
		return nil, err
	}
	return []capiYaml.InitFile{
		*configFile,
	}, nil
}

func (p *ControlPlane) generateKubeadmConfig(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/run/kubeadm/kubeadm.yaml"
	if values.BootstrapToken == "" {
		values.BootstrapToken = uuid.NewString()
	}
	parsedVersion, err := semver.ParseTolerant(values.K8sVersion)
	if err != nil {
		return nil, err
	}
	p.ClusterConfig.ControlPlaneEndpoint = net.JoinHostPort(values.ClusterEndpoint, "6443")
	p.ClusterConfig.ClusterName = values.ClusterName
	p.ClusterConfig.KubernetesVersion = parsedVersion.String()
	p.ClusterConfig.APIServer.CertSANs = []string{"127.0.0.1"}
	initData, err := kubeadmtypes.MarshalInitConfigurationForVersion(&p.ClusterConfig, &p.InitConfig, parsedVersion)
	if err != nil {
		return nil, err
	}
	clusterData, err := kubeadmtypes.MarshalClusterConfigurationForVersion(&p.ClusterConfig, parsedVersion)
	if err != nil {
		return nil, err
	}
	config := strings.Join([]string{clusterData, initData}, "\n---\n")
	return &capiYaml.InitFile{
		Path:        filePath,
		Permissions: "0640",
		Content:     config,
	}, nil
}

func (p *ControlPlane) UpdateManifests(_ context.Context, manifests []string, _ *types.Values) (*capiYaml.ParsedManifest, error) {
	var controlPlane controlplanev1.KubeadmControlPlane
	var controlPlaneManifests capiYaml.ParsedManifest
	for _, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &controlPlane)
		if err != nil {
			return nil, err
		}
		if controlPlane.Kind == "KubeadmControlPlane" {
			for _, file := range controlPlane.Spec.KubeadmConfigSpec.Files {
				newFile := capiYaml.InitFile{
					Path:        file.Path,
					Content:     file.Content,
					Owner:       file.Owner,
					Permissions: file.Permissions,
					Encoding:    string(file.Encoding),
				}
				controlPlaneManifests.AdditionalFiles = append(controlPlaneManifests.AdditionalFiles, newFile)
			}
			for _, cmd := range controlPlane.Spec.KubeadmConfigSpec.PreKubeadmCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PreRunCmd = append(controlPlaneManifests.PreRunCmd, parsedCommand)
			}
			for _, cmd := range controlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PostRunCmd = append(controlPlaneManifests.PostRunCmd, parsedCommand)
			}
		}

	}
	return &controlPlaneManifests, nil
}

func (p *ControlPlane) PreDeploy(_ context.Context, values *types.Values) error {
	// parse the controlPlane from the manifests
	controlPlaneSpec := GetControlPlaneDef(values.Manifests)
	if controlPlaneSpec == nil {
		return errors.New("control plane not found")
	}

	p.ClusterConfig = *controlPlaneSpec.Spec.KubeadmConfigSpec.ClusterConfiguration
	p.InitConfig = *controlPlaneSpec.Spec.KubeadmConfigSpec.InitConfiguration

	// set the k8s version as parsed from the ControlPlane
	values.K8sVersion = controlPlaneSpec.Spec.Version
	klog.Infof("k8s version : %s", controlPlaneSpec.Spec.Version)

	// generate certificates
	p.Certs = secrets.NewCertificatesForInitialControlPlane(&p.ClusterConfig)
	for _, cert := range p.Certs {
		err := cert.Generate()
		if err != nil {
			return err
		}
	}
	// generate kubeconfig
	var serverCACert *x509.Certificate
	var serverCAKey crypto.Signer
	var err error
	for _, cert := range p.Certs {
		switch cert.Purpose {
		case secrets.ClusterCA:
			serverCACert, err = certs.DecodeCertPEM(cert.KeyPair.Cert)
			if err != nil {
				return errors.Join(errors.New("failed to decode server CA certificate"), err)
			}
			serverCAKey, err = certs.DecodePrivateKeyPEM(cert.KeyPair.Key)
			if err != nil {
				return errors.Join(errors.New("failed to decode client CA private key"), err)
			}
		}
	}
	newKubeconfig, err := kubeconfig.New(values.ClusterName, fmt.Sprintf("https://%s", net.JoinHostPort(values.ClusterEndpoint, "6443")), serverCACert, serverCAKey)
	if err != nil {
		return errors.Join(errors.New("failed to generate kubeconfig"), err)
	}
	values.Kubeconfig = &clientv1.Config{}
	err = clientv1.Convert_api_Config_To_v1_Config(newKubeconfig, values.Kubeconfig, nil)
	if err != nil {
		return errors.Join(errors.New("failed to convert kubeconfig to v1"), err)
	}
	// set the BootstrapManifestDir
	values.BootstrapManifestDir = "/var/lib/kubeadm/manifests/"

	return nil
}

func GetControlPlaneDef(manifests []string) *controlplanev1.KubeadmControlPlane {
	var cp controlplanev1.KubeadmControlPlane
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &cp)
		if cp.Kind == "KubeadmControlPlane" {
			return &cp
		}
	}
	return nil
}

func (p *ControlPlane) GetControlPlaneCertSecret(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
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

func (p *ControlPlane) GetControlPlaneCertFiles(_ context.Context) ([]capiYaml.InitFile, error) {
	if p.Certs == nil {
		return nil, ErrNoCerts
	}
	certFiles := p.Certs.AsFiles()
	yamlFiles := make([]capiYaml.InitFile, len(certFiles))
	for i, file := range certFiles {
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

func (p *ControlPlane) GetKubeconfig(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
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
