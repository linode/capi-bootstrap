package cloudinit

import (
	"archive/tar"
	"bytes"
	"capi-bootstrap/providers/controlplane"
	"capi-bootstrap/providers/infrastructure"
	"capi-bootstrap/types"
	"compress/gzip"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	capiYaml "capi-bootstrap/yaml"

	"gopkg.in/yaml.v3"
)

//go:embed files
var files embed.FS

func GenerateCloudInit(ctx context.Context, values *types.Values, infra infrastructure.Provider, controlPlane controlplane.Provider, tarWriteFiles bool) ([]byte, error) {
	debugCmds := []string{"curl -s -L https://github.com/derailed/k9s/releases/download/v0.32.4/k9s_Linux_amd64.tar.gz | tar -xvz -C /usr/local/bin k9s",
		`echo "alias k=\"k3s kubectl\"" >> /root/.bashrc`,
		"echo \"export KUBECONFIG=/etc/rancher/k3s/k3s.yaml\" >> /root/.bashrc"}
	initScriptPath := "/tmp/init-cluster.sh"
	certManager, err := generateCertManagerManifest(values)
	if err != nil {
		return nil, err
	}
	capiOperator, err := generateCapiOperator(values)
	if err != nil {
		return nil, err
	}

	capiManifests, err := GenerateCapiManifests(ctx, values, infra, controlPlane, true)
	if err != nil {
		return nil, err
	}

	// infra specific
	additionalInfraFiles, err := infra.GenerateAdditionalFiles(ctx, values)
	if err != nil {
		return nil, err
	}
	infraCapi, err := infra.GenerateCapiFile(ctx, values)
	if err != nil {
		return nil, err
	}
	capiPivotMachine, err := infra.GenerateCapiMachine(ctx, values)
	if err != nil {
		return nil, err
	}
	// control plane specific
	controlPlaneCapi, err := controlPlane.GenerateCapiFile(ctx, values)
	if err != nil {
		return nil, err
	}
	additionalControlPlaneFiles, err := controlPlane.GenerateAdditionalFiles(ctx, values)
	if err != nil {
		return nil, err
	}
	initScript, err := controlPlane.GenerateInitScript(ctx, initScriptPath, values)
	if err != nil {
		return nil, err
	}
	controlPlaneRunCmd, err := controlPlane.GenerateRunCommand(ctx, values)
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
		*initScript,
	}
	writeFiles = append(writeFiles, additionalInfraFiles...)
	writeFiles = append(writeFiles, additionalControlPlaneFiles...)
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

func generateCertManagerManifest(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "cert-manager.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "cert-manager.yaml"), files, values, false)
}

func generateCapiOperator(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-operator.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-operator.yaml"), files, values, false)
}

func GenerateCapiManifests(ctx context.Context, values *types.Values, infra infrastructure.Provider, controlPlane controlplane.Provider, escapeYaml bool) (*capiYaml.ParsedManifest, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-manifests.yaml")
	cloudInitFile, err := capiYaml.ConstructFile(filePath, values.ManifestFile, values.ManifestFS, values, escapeYaml)
	if err != nil {
		return nil, err
	}
	var capiManifests *capiYaml.ParsedManifest
	initFileContent, capiManifests, err := UpdateManifest(ctx, cloudInitFile.Content, infra, controlPlane, values)
	if err != nil {
		return nil, err
	}
	cloudInitFile.Content = string(initFileContent)
	capiManifests.ManifestFile = cloudInitFile
	return capiManifests, nil
}

func createTar(cloudFiles []capiYaml.InitFile) (io.Reader, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	defer func(gzipWriter *gzip.Writer) {
		err := gzipWriter.Close()
		if err != nil {
			return
		}
	}(gzipWriter)
	tarWriter := tar.NewWriter(gzipWriter)
	defer func(tarWriter *tar.Writer) {
		err := tarWriter.Close()
		if err != nil {
			return
		}
	}(tarWriter)
	for _, file := range cloudFiles {
		header := &tar.Header{
			Name:    file.Path[1:],
			Size:    int64(len(file.Content)),
			ModTime: time.Now(),
			Mode:    0o644,
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

func UpdateManifest(ctx context.Context, yamlManifest string, infra infrastructure.Provider, controlPlane controlplane.Provider, values *types.Values) ([]byte, *capiYaml.ParsedManifest, error) {
	manifests := strings.Split(yamlManifest, "---")
	controlPlaneManifests := &capiYaml.ParsedManifest{}
	var err error
	if err := capiYaml.UpdateCluster(manifests); err != nil {
		return nil, nil, err
	}
	if infra != nil {
		if err := infra.UpdateManifests(ctx, manifests, values); err != nil {
			return nil, nil, err
		}
	}

	if controlPlane != nil {
		controlPlaneManifests, err = controlPlane.UpdateManifests(ctx, manifests, values)
		if err != nil {
			return nil, nil, err
		}
	}

	yamlManifest = strings.Join(manifests, "---\n")
	return []byte(yamlManifest), controlPlaneManifests, nil
}
