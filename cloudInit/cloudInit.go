package cloudInit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
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
	runCmds := []string{
		fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q sh -", values.K8sVersion),
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
		*initScript,
	}
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

//go:embed files
var files embed.FS

func generateCertManagerManifest(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/cert-manager.yaml"
	return constructFile(filePath, "files/cert-manager.yaml", files, values)
}

func generateCapiOperator(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-operator.yaml"
	return constructFile(filePath, "files/capi-operator.yaml", files, values)
}

func generateLinodeCCM(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/linode-ccm.yaml"
	return constructFile(filePath, "files/linode/linode-ccm.yaml", files, values)
}

func generateK3sProvider(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-k3s.yaml"
	return constructFile(filePath, "files/k3s/capi-k3s.yaml", files, values)
}

func generateCapiLinode(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-linode.yaml"
	return constructFile(filePath, "files/linode/capi-linode.yaml", files, values)
}

func generateK3sConfig(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/etc/rancher/k3s/config.yaml"
	return constructFile(filePath, "files/k3s/config.yaml", files, values)
}

func generateCapiPivotMachine(values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-pivot-machine.yaml"
	return constructFile(filePath, "files/capi-pivot-machine.yaml", files, values)
}

func GenerateCapiManifests(values capiYaml.Substitutions) (*capiYaml.ParsedManifest, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-pivot-k3s.yaml"
	cloudInitFile, err := constructFile(filePath, "files/capi-pivot-manifests.yaml", files, values)
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
	return constructFile(filePath, "files/init-cluster.sh", files, values)
}

func templateManifest(filesystem fs.FS, localPath string, templateValues capiYaml.Substitutions) ([]byte, error) {
	var err error
	tmpl := template.New(filepath.Base(localPath))
	tmpl.Delims("{{{", "}}}")
	tmpl, err = tmpl.ParseFS(filesystem, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s, %s", localPath, err)
	}

	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, templateValues)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template %s, %s", localPath, err)
	}
	return buf.Bytes(), nil
}

func constructFile(filePath string, localPath string, filesystem fs.FS, values capiYaml.Substitutions) (*capiYaml.InitFile, error) {
	manifest, err := templateManifest(filesystem, localPath, values)
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
