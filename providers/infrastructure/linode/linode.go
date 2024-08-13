package linode

import (
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	caplv1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/linodego"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"
)

// LinodeClient is an interface that includes all linodego calls, so they can be mocked out for testing
type LinodeClient interface {
	ListNodeBalancers(ctx context.Context, opts *linodego.ListOptions) ([]linodego.NodeBalancer, error)
	CreateNodeBalancer(ctx context.Context, opts linodego.NodeBalancerCreateOptions) (*linodego.NodeBalancer, error)
	CreateNodeBalancerConfig(ctx context.Context, nodebalancerID int, opts linodego.NodeBalancerConfigCreateOptions) (*linodego.NodeBalancerConfig, error)
	DeleteNodeBalancer(ctx context.Context, nodebalancerID int) error
	CreateNodeBalancerNode(ctx context.Context, nodebalancerID int, configID int, opts linodego.NodeBalancerNodeCreateOptions) (*linodego.NodeBalancerNode, error)
	ListVPCs(ctx context.Context, opts *linodego.ListOptions) ([]linodego.VPC, error)
	CreateVPC(ctx context.Context, opts linodego.VPCCreateOptions) (*linodego.VPC, error)
	DeleteVPC(ctx context.Context, vpcID int) error
	CreateInstance(ctx context.Context, opts linodego.InstanceCreateOptions) (*linodego.Instance, error)
	ListInstances(ctx context.Context, opts *linodego.ListOptions) ([]linodego.Instance, error)
	DeleteInstance(ctx context.Context, linodeID int) error
}

type Infrastructure struct {
	Name               string
	Client             LinodeClient `json:"-"`
	Machine            *v1alpha1.LinodeMachineTemplate
	NodeBalancer       *linodego.NodeBalancer
	NodeBalancerConfig *linodego.NodeBalancerConfig
	Token              string
	AuthorizedKeys     string
	VPC                *v1alpha1.LinodeVPC
}

func NewInfrastructure() *Infrastructure {
	return &Infrastructure{
		Name: "LinodeCluster",
	}
}

func (p *Infrastructure) GenerateCapiFile(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-linode.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-linode.yaml"), files, p.getTemplateValues(values), false)
}

func (p *Infrastructure) GenerateCapiMachine(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-pivot-machine.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-pivot-machine.yaml"), files, p.getTemplateValues(values), false)
}

func (p *Infrastructure) GenerateAdditionalFiles(ctx context.Context, values *types.Values) ([]capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "linode-ccm.yaml")
	localPath := filepath.Join("files", "linode-ccm.yaml")
	if p.VPC != nil {
		localPath = filepath.Join("files", "linode-ccm-vpc.yaml")
	}
	CCMFile, err := capiYaml.ConstructFile(filePath, localPath, files, p.getTemplateValues(values), false)
	if err != nil {
		return nil, err
	}
	return []capiYaml.InitFile{*CCMFile}, nil
}

func (p *Infrastructure) PreCmd(ctx context.Context, values *types.Values) error {
	p.Token = os.Getenv("LINODE_TOKEN")

	if p.Token == "" {
		return errors.New("LINODE_TOKEN env variable is required")
	}
	client := NewClient(p.Token, ctx)
	p.Client = &client
	return nil
}

func (p *Infrastructure) PreDeploy(ctx context.Context, values *types.Values) error {
	p.Machine = GetLinodeMachineDef(values.Manifests)
	if p.Machine == nil {
		return errors.New("machine not found")
	}

	nbListFilter, err := json.Marshal(map[string]string{"tags": values.ClusterName})
	if err != nil {
		return fmt.Errorf("unable to unmarshal nodebalancer list filter: %s", err)
	}
	existingNB, err := p.Client.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(nbListFilter)))
	if err != nil {
		return fmt.Errorf("unable to list NodeBalancers: %s", err)
	}
	if len(existingNB) != 0 {
		return errors.New("node balancer already exists")
	}
	// Create a NodeBalancer
	p.NodeBalancer, err = p.Client.CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
		Label:  &values.ClusterName,
		Region: p.Machine.Spec.Template.Spec.Region,
		Tags:   []string{values.ClusterName},
	})
	if err != nil {
		return fmt.Errorf("unable to create NodeBalancer: %s", err)
	}
	klog.Infof("Created NodeBalancer: %v\n", *p.NodeBalancer.Label)

	// Create a NodeBalancer Config
	p.NodeBalancerConfig, err = p.Client.CreateNodeBalancerConfig(ctx, p.NodeBalancer.ID, linodego.NodeBalancerConfigCreateOptions{
		Port:      6443,
		Protocol:  "tcp",
		Algorithm: "roundrobin",
		Check:     "connection",
	})
	if err != nil {
		return fmt.Errorf("unable to unmarshal nodebalancer list filter: %s", err)
	}

	if p.NodeBalancer.IPv4 == nil {
		return errors.New("no node IPv4 address on NodeBalancer")
	}

	p.AuthorizedKeys = os.Getenv("AUTHORIZED_KEYS")
	values.ClusterEndpoint = *p.NodeBalancer.IPv4

	if vpcDef := GetVPCRef(values.Manifests); vpcDef != nil {
		p.VPC = vpcDef
	}
	return nil
}

func (p *Infrastructure) Deploy(ctx context.Context, values *types.Values, metadata []byte) error {
	createOptions := linodego.InstanceCreateOptions{
		Label:     values.ClusterName + "-bootstrap",
		Image:     p.Machine.Spec.Template.Spec.Image,
		Region:    p.Machine.Spec.Template.Spec.Region,
		Type:      p.Machine.Spec.Template.Spec.Type,
		RootPass:  uuid.NewString(),
		Tags:      []string{values.ClusterName},
		PrivateIP: true,
		Metadata:  &linodego.InstanceMetadataOptions{UserData: base64.StdEncoding.EncodeToString(metadata)},
	}

	if p.VPC != nil {
		var vpc *linodego.VPC
		var vpcSubnets []linodego.VPCSubnetCreateOptions
		for _, subnet := range p.VPC.Spec.Subnets {
			vpcSubnets = append(vpcSubnets, linodego.VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
			})
		}
		vpc, err := p.Client.CreateVPC(ctx, linodego.VPCCreateOptions{
			Label:       p.VPC.Name,
			Description: p.VPC.Spec.Description,
			Region:      p.VPC.Spec.Region,
			Subnets:     vpcSubnets,
		})
		if err != nil {
			return fmt.Errorf("unable to create VPC: %s", err)
		}
		natAny := "any"
		createOptions.Interfaces = []linodego.InstanceConfigInterfaceCreateOptions{
			{
				Purpose:  linodego.InterfacePurposeVPC,
				Primary:  true,
				SubnetID: &vpc.Subnets[0].ID,
				IPv4: &linodego.VPCIPv4{
					NAT1To1: &natAny,
				}},
			{Purpose: linodego.InterfacePurposePublic}}
	}

	if p.AuthorizedKeys != "" {
		createOptions.AuthorizedKeys = []string{p.AuthorizedKeys}
	}

	// Create a Linode Instance
	instance, err := p.Client.CreateInstance(ctx, createOptions)
	if err != nil {
		return fmt.Errorf("unable to create Instance: %s", err)
	}

	klog.Infof("Created Linode Instance: %v\n", instance.Label)

	var privateIP string

	for _, ip := range instance.IPv4 {
		if ip.IsPrivate() {
			privateIP = ip.String()
		}
	}

	// Create a NodeBalancer Node
	node, err := p.Client.CreateNodeBalancerNode(ctx, p.NodeBalancer.ID, p.NodeBalancerConfig.ID, linodego.NodeBalancerNodeCreateOptions{
		Address: fmt.Sprintf("%s:6443", privateIP),
		Label:   values.ClusterName + "-bootstrap",
		Weight:  100,
	})
	if err != nil {
		return err
	}

	klog.Infof("Created NodeBalancer Node: %v\n", node.Label)
	klog.Infof("Bootstrap Node IP: %s\n", instance.IPv4[0].String())
	return nil
}

func (p *Infrastructure) PostDeploy(ctx context.Context, values *types.Values) error {
	// Not currently used by the LinodeProvider
	return nil
}

func (p *Infrastructure) Delete(ctx context.Context, values *types.Values, force bool) error {
	ListFilter, err := json.Marshal(map[string]string{"tags": values.ClusterName})
	if err != nil {
		return err
	}

	instances, err := p.Client.ListInstances(ctx, &linodego.ListOptions{
		Filter: string(ListFilter),
	})
	if err != nil {
		return fmt.Errorf("could not list instances: %v", err)
	}

	if len(instances) > 0 {
		klog.Info("Will delete instances:\n")
		for _, instance := range instances {
			klog.Infof("  Label: %s, ID: %d\n", instance.Label, instance.ID)
		}
	}

	VPCListFilter, err := json.Marshal(map[string]string{"label": values.ClusterName})
	if err != nil {
		return fmt.Errorf("could construct VPC filter: %v", err)
	}
	vpcs, err := p.Client.ListVPCs(ctx, &linodego.ListOptions{
		Filter: string(VPCListFilter),
	})
	if err != nil {
		return fmt.Errorf("could not list VPCs: %v", err)
	}

	if len(vpcs) > 0 {
		klog.Info("Will delete vpc:\n")
		for _, vpc := range vpcs {
			klog.Infof("  Label: %s, ID: %d\n", vpc.Label, vpc.ID)
		}
	}

	nodeBal, err := p.Client.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(ListFilter)))
	if err != nil {
		return fmt.Errorf("could not list NodeBalancers: %v", err)
	}
	switch len(nodeBal) {
	case 1:
		klog.Infof("Will delete NodeBalancer:\n")
		klog.Infof("  Label: %s, ID: %d\n", *nodeBal[0].Label, nodeBal[0].ID)

	case 0:
		klog.Infof("No NodeBalancers found for deletion")
	default:
		klog.Fatalf("More than one NodeBalaner found for deletion, cannot delete")
	}
	var confirm string
	if !force {
		klog.Info("Would you like to delete these resources(y/n): ")
		if _, err := fmt.Scanln(&confirm); err != nil {
			return errors.New("error trying to read user input")
		}
		if confirm != "y" && confirm != "yes" {
			return nil
		}
	}

	klog.Info("Deleting resources:")

	for _, instance := range instances {
		if err := p.Client.DeleteInstance(ctx, instance.ID); err != nil {
			return fmt.Errorf("could not delete Instance %s: %v", instance.Label, err)
		}
		klog.Infof("  Deleted Instance %s\n", instance.Label)
	}

	if len(nodeBal) == 1 {
		if err := p.Client.DeleteNodeBalancer(ctx, nodeBal[0].ID); err != nil {
			return fmt.Errorf("could not delete NodeBalancer %s: %v", *nodeBal[0].Label, err)
		}
		klog.Infof("  Deleted NodeBalancer %s\n", *nodeBal[0].Label)
	}

	if len(vpcs) == 1 {
		if err := p.Client.DeleteVPC(ctx, vpcs[0].ID); err != nil {
			return fmt.Errorf("could not delete VPC %s: %v", vpcs[0].Label, err)
		}
		klog.Infof("  Deleted VPC %s\n", *nodeBal[0].Label)
	}

	return nil
}

func GetLinodeMachineDef(manifests []string) *caplv1alpha1.LinodeMachineTemplate {
	var template caplv1alpha1.LinodeMachineTemplate
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &template)
		if template.Kind == "LinodeMachineTemplate" {
			return &template
		}
	}
	return nil
}

func (p *Infrastructure) UpdateManifests(ctx context.Context, manifests []string, values *types.Values) error {
	var LinodeClusterIndex int
	var LinodeCluster v1alpha2.LinodeCluster
	for i, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &LinodeCluster)
		if err != nil {
			return err
		}
		if LinodeCluster.Kind == "LinodeCluster" {
			LinodeCluster.Spec.ControlPlaneEndpoint = v1beta1.APIEndpoint{
				Host: values.ClusterEndpoint,
				Port: int32(p.NodeBalancerConfig.Port),
			}
			LinodeCluster.Spec.Network = v1alpha2.NetworkSpec{
				LoadBalancerType:              "NodeBalancer",
				ApiserverLoadBalancerPort:     6443,
				NodeBalancerID:                &p.NodeBalancer.ID,
				ApiserverNodeBalancerConfigID: &p.NodeBalancerConfig.ID,
			}
			LinodeClusterIndex = i
			break
		}
	}
	LinodeClusterString, err := yaml.Marshal(LinodeCluster)
	if err != nil {
		return err
	}
	manifests[LinodeClusterIndex] = string(LinodeClusterString)
	return nil
}

func GetVPCRef(manifests []string) *caplv1alpha1.LinodeVPC {
	var vpc caplv1alpha1.LinodeVPC
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &vpc)
		if vpc.Kind == "LinodeVPC" {
			return &vpc
		}
	}
	return nil
}

func (p *Infrastructure) getTemplateValues(v *types.Values) any {
	return struct {
		*types.Values
		Linode *Infrastructure
	}{
		v,
		p,
	}
}
