package cloudinit

import (
	"archive/tar"
	"bytes"
	"capi-bootstrap/helm"
	"capi-bootstrap/providers"
	"compress/gzip"
	"context"
	"embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

//go:embed files
var files embed.FS

func GenerateCloudInit(ctx context.Context, values *types.Values, providers providers.Providers) ([]byte, error) {
	debugCmds := []string{"curl -s -L https://github.com/derailed/k9s/releases/download/v0.32.4/k9s_Linux_amd64.tar.gz | tar -xvz -C /usr/local/bin k9s",
		`echo "alias k=\"kubectl\"" >> /root/.bashrc`}
	initScriptPath := "/tmp/init-cluster.sh"
	certManager, err := generateCertManagerManifest(values)
	if err != nil {
		return nil, err
	}
	capiOperator, err := generateCapiOperator(values)
	if err != nil {
		return nil, err
	}
	capiManifests, err := GenerateCapiManifests(ctx, values, providers)
	if err != nil {
		return nil, err
	}

	// infra specific
	additionalInfraFiles, err := providers.Infrastructure.GenerateAdditionalFiles(ctx, values)
	if err != nil {
		return nil, err
	}
	infraCapi, err := providers.Infrastructure.GenerateCapiFile(ctx, values)
	if err != nil {
		return nil, err
	}
	capiPivotMachine, err := providers.Infrastructure.GenerateCapiMachine(ctx, values)
	if err != nil {
		return nil, err
	}

	// control plane specific
	controlPlaneCapi, err := providers.ControlPlane.GenerateCapiFile(ctx, values)
	if err != nil {
		return nil, err
	}
	additionalControlPlaneFiles, err := providers.ControlPlane.GenerateAdditionalFiles(ctx, values)
	if err != nil {
		return nil, err
	}
	initScript, err := providers.ControlPlane.GenerateInitScript(ctx, initScriptPath, values)
	if err != nil {
		return nil, err
	}
	controlPlaneRunCmd, err := providers.ControlPlane.GenerateRunCommand(ctx, values)
	if err != nil {
		return nil, err
	}
	controlPlaneCertFiles, err := providers.ControlPlane.GetControlPlaneCertFiles(ctx)
	if err != nil {
		return nil, err
	}
	controlPlaneCertSecrets, err := providers.ControlPlane.GetControlPlaneCertSecret(ctx, values)
	if err != nil {
		return nil, err
	}
	kubeconfigSecret, err := providers.ControlPlane.GetKubeconfig(ctx, values)
	if err != nil {
		return nil, err
	}

	runCmds := []string{fmt.Sprintf("bash %s", initScriptPath)}
	runCmds = append(debugCmds, runCmds...)
	runCmds = append(controlPlaneRunCmd, runCmds...)
	runCmds = append(capiManifests.PreRunCmd, runCmds...)
	runCmds = append(runCmds, capiManifests.PostRunCmd...)

	writeFiles := []capiYaml.InitFile{
		*certManager,
		*capiOperator,
		*controlPlaneCapi,
		*infraCapi,
		*capiPivotMachine,
		*capiManifests.ManifestFile,
		*controlPlaneCertSecrets,
		*kubeconfigSecret,
		*initScript,
	}
	writeFiles = append(writeFiles, additionalInfraFiles...)
	writeFiles = append(writeFiles, additionalControlPlaneFiles...)
	writeFiles = append(writeFiles, controlPlaneCertFiles...)
	writeFiles = append(writeFiles, capiManifests.AdditionalFiles...)
	if values.TarWriteFiles {
		writeFiles, err = createTar(writeFiles)
		if err != nil {
			return nil, err
		}
		runCmds = append([]string{"tar -C / -xvf /tmp/cloud-init-files.tgz", "tar -xf /tmp/cloud-init-files.tgz --to-command='xargs -0 cloud-init query -f > /$TAR_FILENAME'"}, runCmds...)
	}

	cloudConfig := capiYaml.Config{
		WriteFiles: writeFiles,
		RunCmd:     runCmds,
	}
	downloadCmds, err := providers.Backend.WriteFiles(ctx, values.ClusterName, &cloudConfig)
	if err != nil {
		return nil, err
	}
	cloudConfig.RunCmd = append(downloadCmds, cloudConfig.RunCmd...)
	rawCloudConfig, err := yaml.Marshal(cloudConfig)
	if err != nil {
		return nil, err
	}

	renderedCloudConfig := append([]byte("## template: jinja\n#cloud-config\n\n"), rawCloudConfig...)
	return renderedCloudConfig, nil
}

func generateCertManagerManifest(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "cert-manager.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "cert-manager.yaml"), files, values, false)
}

func generateCapiOperator(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-operator.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-operator.yaml"), files, values, false)
}

func GenerateCapiManifests(ctx context.Context, values *types.Values, providers providers.Providers) (*capiYaml.ParsedManifest, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-manifests.yaml")

	capiManifests, err := UpdateManifest(ctx, providers, values)
	if err != nil {
		return nil, err
	}
	helmFiles, err := helm.AddHelmCharts(values)
	if err != nil {
		return nil, err
	}
	capiManifests.AdditionalFiles = append(capiManifests.AdditionalFiles, helmFiles...)
	rawYaml := strings.Join(values.Manifests, "---\n")
	manifest, err := capiYaml.TemplateManifest([]byte(rawYaml), values.ManifestFile, values, true)
	if err != nil {
		return nil, err
	}
	values.Manifests = strings.Split(string(manifest), "---")
	cloudInitFile := capiYaml.InitFile{
		Path: filePath,
	}
	cloudInitFile.Content = string(manifest)
	capiManifests.ManifestFile = &cloudInitFile
	return capiManifests, nil
}

func createTar(cloudFiles []capiYaml.InitFile) ([]capiYaml.InitFile, error) {
	data, err := tarFromInitFiles(cloudFiles)
	if err != nil {
		return nil, err
	}

	writeFiles := []capiYaml.InitFile{{
		Path:    "/tmp/cloud-init-files.tgz",
		Content: string(data),
	}}
	return writeFiles, nil
}

func tarFromInitFiles(files []capiYaml.InitFile) (data []byte, err error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	defer func() {
		err = tarWriter.Close() // close tar writer first
		if err != nil {
			return
		}
		err = gzipWriter.Close() // close gzip writer second
		if err != nil {
			return
		}
		data, err = io.ReadAll(&buf) // capture all output
	}()

	for _, file := range files {
		header := &tar.Header{
			Name:    file.Path[1:],
			Size:    int64(len(file.Content)),
			ModTime: time.Now(),
			Mode:    0o644,
		}
		err = tarWriter.WriteHeader(header)
		if err != nil {
			return data, err
		}

		_, err = io.WriteString(tarWriter, file.Content)
		if err != nil {
			return data, err
		}
	}

	return data, err
}

func UpdateManifest(ctx context.Context, providers providers.Providers, values *types.Values) (*capiYaml.ParsedManifest, error) {
	controlPlaneManifests := &capiYaml.ParsedManifest{}
	var err error
	if err := capiYaml.UpdateCluster(values.Manifests); err != nil {
		return nil, err
	}
	if providers.Infrastructure != nil {
		if err := providers.Infrastructure.UpdateManifests(ctx, values.Manifests, values); err != nil {
			return nil, err
		}
	}

	if providers.ControlPlane != nil {
		controlPlaneManifests, err = providers.ControlPlane.UpdateManifests(ctx, values.Manifests, values)
		if err != nil {
			return nil, err
		}
	}

	return controlPlaneManifests, nil
}
