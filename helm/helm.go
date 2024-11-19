package helm

import (
	"bytes"
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"fmt"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"

	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	addonsv1alpha1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

type Templates struct {
	Cluster      clusterv1.Cluster
	InfraCluster map[string]any
}
type HelmFile struct {
	Repositories []HelmRepos `json:"repositories"`

	Releases []HelmRelease `json:"releases"`
}
type HelmRepos struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type HelmRelease struct {
	Name                              string `json:"name"`
	Version                           string `json:"version"`
	addonsv1alpha1.HelmOptions        `json:",inline"`
	addonsv1alpha1.HelmInstallOptions `json:",inline"`
	Values                            map[string]any `json:"values"`
}

func BuildTemplateValues(values types.Values) (*Templates, error) {
	helmValues := Templates{}
	// find cluster resource and marshall it into templating values
	for _, manifest := range values.Manifests {
		err := yaml.Unmarshal([]byte(manifest), &helmValues.Cluster)
		if err != nil {
			return nil, err
		}
		if helmValues.Cluster.Kind == "Cluster" {
			break
		}
	}
	// find infraCluster by checking against values.ClusterKind
	for _, manifest := range values.Manifests {
		err := yaml.Unmarshal([]byte(manifest), &helmValues.InfraCluster)
		if err != nil {
			return nil, err
		}
		if helmValues.InfraCluster["kind"] == values.ClusterKind {
			break
		}
	}
	return &helmValues, nil

}
func ProxyToHelmFile(proxy addonsv1alpha1.HelmChartProxy) ([]byte, error) {

	helmFile := HelmFile{
		Repositories: []HelmRepos{{
			Name: proxy.Name,
			URL:  proxy.Spec.RepoURL,
		}},
		Releases: []HelmRelease{{
			Name:        proxy.Spec.ReleaseName,
			Version:     proxy.Spec.Version,
			HelmOptions: proxy.Spec.Options,
		}},
	}

	err := yaml.Unmarshal([]byte(proxy.Spec.ValuesTemplate), helmFile.Releases[0].Values)
	if err != nil {
		return nil, err
	}
	helmFileString, err := yaml.Marshal(helmFile)
	return helmFileString, nil

}

func AddHelmCharts(values *types.Values) ([]capiYaml.InitFile, error) {
	helmValues, err := BuildTemplateValues(*values)
	if err != nil {
		return nil, err
	}
	proxy := addonsv1alpha1.HelmChartProxy{}
	helmInstalls := []string{"curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"}
	var helmFiles []capiYaml.InitFile
	for _, manifest := range values.Manifests {
		err := yaml.Unmarshal([]byte(manifest), &proxy)
		if err != nil {
			return nil, err
		}
		if proxy.Kind == "HelmChartProxy" {
			newValues, err := templateValues(proxy.Spec.ValuesTemplate, *helmValues)
			if err != nil {
				return nil, err
			}
			proxy.Spec.ValuesTemplate = string(newValues)
			helmFile, err := ProxyToHelmFile(proxy)
			if err != nil {
				return nil, err
			}
			helmFiles = append(helmFiles, capiYaml.InitFile{Path: fmt.Sprintf("/var/lib/kubeadm/helm/%s-helmfile.yaml", proxy.Name), Content: string(helmFile)})
			// helmInstall, err := ProxyToHelmInstall(&proxy)
			// if err != nil {
			// 	return nil, err
			// }
			// helmInstalls = append(helmInstalls, helmInstall)
		}
	}
	helmFiles = append(helmFiles, capiYaml.InitFile{Path: "/tmp/helm-install.sh", Content: strings.Join(helmInstalls, "\n")})

	return helmFiles, nil
}

func templateValues(values string, helmValues Templates) ([]byte, error) {
	tmpl, err := template.New("values").Parse(values)
	if err != nil {
		return nil, err
	}
	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, helmValues)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}

func buildHelmOptions(options addonsv1alpha1.HelmOptions) string {
	optionString := ""
	if options.DisableHooks {
		optionString += " --disable-hooks"
	}
	if options.Atomic {
		optionString += " --atomic"
	}
	if options.DependencyUpdate {
		optionString += " --dependency-update"
	}
	if options.Wait {
		optionString += " --wait"
	}
	if options.SkipCRDs {
		optionString += " --skip-crds"
	}
	if options.DisableOpenAPIValidation {
		optionString += " --disable-openapi-validation"
	}
	if options.EnableClientCache {
		optionString += " --enable-client-cache"
	}
	if options.WaitForJobs {
		optionString += " --wait-for-jobs"
	}
	if options.Install.IncludeCRDs {
		optionString += " --include-crds"
	}
	if options.Install.CreateNamespace {
		optionString += " --create-namespace"
	}
	return optionString
}
func ProxyToHelmInstall(proxy *addonsv1alpha1.HelmChartProxy) (string, error) {
	helmInstall := "helm upgrade --install {{ .Spec.ChartName }} {{ .Spec.ChartName }} --repo {{ .Spec.RepoURL }}  -n {{ .Spec.ReleaseNamespace}} --version {{ .Spec.Version }} -f /var/lib/kubeadm/helm/{{ .Name }}-values.yaml"
	helmInstall += buildHelmOptions(proxy.Spec.Options)
	tmpl, err := template.New("helmCommand").Parse(helmInstall)
	if err != nil {
		return "", err
	}
	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, proxy)
	if err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil

}
func ProxyToChart(proxy addonsv1alpha1.HelmChartProxy) *helmv1.HelmChart {
	chart := helmv1.HelmChart{
		TypeMeta:   controllerruntime.TypeMeta{Kind: "HelmChart", APIVersion: "helm.cattle.io/v1"},
		ObjectMeta: proxy.ObjectMeta,
		Spec: helmv1.HelmChartSpec{
			TargetNamespace: proxy.Spec.ReleaseNamespace,
			CreateNamespace: proxy.Spec.Options.Install.CreateNamespace,
			Chart:           proxy.Spec.ChartName,
			Version:         proxy.Spec.Version,
			Repo:            proxy.Spec.RepoURL,
			ValuesContent:   proxy.Spec.ValuesTemplate,
			Bootstrap:       true,
			Timeout:         proxy.Spec.Options.Timeout,
		},
		Status: helmv1.HelmChartStatus{},
	}
	chart.Namespace = proxy.Spec.ReleaseNamespace
	if proxy.Spec.ReleaseName == "" {
		chart.Name = proxy.Spec.ChartName
	} else {
		chart.Name = proxy.Spec.ReleaseName
	}

	return &chart
}
