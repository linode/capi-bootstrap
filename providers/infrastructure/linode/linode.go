package Linode

import (
	"capi-bootstrap/providers"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	caplv1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/linodego"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"
)

type CAPL struct{}

func (CAPL) Name() string {
	return "linode-linode"
}

func (CAPL) GenerateCapiFile(ctx context.Context, values providers.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-linode.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-linode.yaml"), files, values, false)
}

func (CAPL) GenerateCapiMachine(ctx context.Context, values providers.Values) (*capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "capi-pivot-machine.yaml")
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-pivot-machine.yaml"), files, values, false)
}

func (CAPL) GenerateAdditionalFiles(ctx context.Context, values providers.Values) ([]capiYaml.InitFile, error) {
	filePath := filepath.Join(values.BootstrapManifestDir, "linode-ccm.yaml")
	localPath := filepath.Join("files", "linode-ccm.yaml")
	if values.Linode.VPC != nil {
		localPath = filepath.Join("files", "linode-ccm-vpc.yaml")
	}
	CCMFile, err := capiYaml.ConstructFile(filePath, localPath, files, values, false)
	if err != nil {
		return nil, err
	}
	return []capiYaml.InitFile{*CCMFile}, nil
}

func (CAPL) PreCmd(ctx context.Context, values *providers.Values) error {
	values.Linode.Token = os.Getenv("LINODE_TOKEN")

	if values.Linode.Token == "" {
		return errors.New("LINODE_TOKEN env variable is required")
	}

	values.Linode.Client = Client(values.Linode.Token, ctx)
	return nil
}

func (CAPL) PreDeploy(ctx context.Context, values *providers.Values) error {
	values.Linode.Machine = GetLinodeMachineDef(values.Manifests)
	if values.Linode.Machine == nil {
		return errors.New("machine not found")
	}

	nbListFilter, err := json.Marshal(map[string]string{"tags": values.ClusterName})
	if err != nil {
		return err
	}
	existingNB, err := values.Linode.Client.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(nbListFilter)))
	if err != nil {
		return err
	}
	if len(existingNB) != 0 {
		return errors.New("node balancer already exists")
	}
	// Create a NodeBalancer
	values.Linode.NodeBalancer, err = values.Linode.Client.CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
		Label:  &values.ClusterName,
		Region: values.Linode.Machine.Spec.Template.Spec.Region,
		Tags:   []string{values.ClusterName},
	})
	if err != nil {
		return err
	}
	klog.Infof("Created NodeBalancer: %v\n", *values.Linode.NodeBalancer.Label)

	// Create a NodeBalancer Config
	values.Linode.NodeBalancerConfig, err = values.Linode.Client.CreateNodeBalancerConfig(ctx, values.Linode.NodeBalancer.ID, linodego.NodeBalancerConfigCreateOptions{
		Port:      6443,
		Protocol:  "tcp",
		Algorithm: "roundrobin",
		Check:     "connection",
	})
	if err != nil {
		return err
	}

	if values.Linode.NodeBalancer.IPv4 == nil {
		return errors.New("no node IPv4 address on NodeBalancer")
	}

	values.Linode.AuthorizedKeys = os.Getenv("AUTHORIZED_KEYS")
	values.ClusterEndpoint = *values.Linode.NodeBalancer.IPv4

	if vpcDef := GetVPCRef(values.Manifests); vpcDef != nil {
		values.Linode.VPC = vpcDef
	}
	return nil
}

func (CAPL) Deploy(ctx context.Context, values *providers.Values, metadata []byte) error {
	createOptions := linodego.InstanceCreateOptions{
		Label:     values.ClusterName + "-bootstrap",
		Image:     values.Linode.Machine.Spec.Template.Spec.Image,
		Region:    values.Linode.Machine.Spec.Template.Spec.Region,
		Type:      values.Linode.Machine.Spec.Template.Spec.Type,
		RootPass:  uuid.NewString(),
		Tags:      []string{values.ClusterName},
		PrivateIP: true,
		Metadata:  &linodego.InstanceMetadataOptions{UserData: base64.StdEncoding.EncodeToString(metadata)},
	}

	if values.Linode.VPC != nil {
		var vpc *linodego.VPC
		var vpcSubnets []linodego.VPCSubnetCreateOptions
		for _, subnet := range values.Linode.VPC.Spec.Subnets {
			vpcSubnets = append(vpcSubnets, linodego.VPCSubnetCreateOptions{
				Label: subnet.Label,
				IPv4:  subnet.IPv4,
			})
		}
		vpc, err := values.Linode.Client.CreateVPC(ctx, linodego.VPCCreateOptions{
			Label:       values.Linode.VPC.Name,
			Description: values.Linode.VPC.Spec.Description,
			Region:      values.Linode.VPC.Spec.Region,
			Subnets:     vpcSubnets,
		})
		if err != nil {
			return errors.New("Unable to create VPC: " + err.Error())
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

	if values.Linode.AuthorizedKeys != "" {
		createOptions.AuthorizedKeys = []string{values.Linode.AuthorizedKeys}
	}

	// Create a Linode Instance
	instance, err := values.Linode.Client.CreateInstance(ctx, createOptions)
	if err != nil {
		return err
	}

	klog.Infof("Created Linode Instance: %v\n", instance.Label)

	var privateIP string

	for _, ip := range instance.IPv4 {
		if ip.IsPrivate() {
			privateIP = ip.String()
		}
	}

	// Create a NodeBalancer Node
	node, err := values.Linode.Client.CreateNodeBalancerNode(ctx, values.Linode.NodeBalancer.ID, values.Linode.NodeBalancerConfig.ID, linodego.NodeBalancerNodeCreateOptions{
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

func (CAPL) PostDeploy(ctx context.Context, values *providers.Values) error {
	// Not currently used by the LinodeProvider
	return nil
}

func (CAPL) Delete(ctx context.Context, values *providers.Values, force bool) error {
	values.Linode.Client = Client(values.Linode.Token, ctx)
	ListFilter, err := json.Marshal(map[string]string{"tags": values.ClusterName})
	if err != nil {
		return err
	}

	instances, err := values.Linode.Client.ListInstances(ctx, ptr.To(linodego.ListOptions{
		Filter: string(ListFilter),
	}))
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
	vpcs, err := values.Linode.Client.ListVPCs(ctx, ptr.To(linodego.ListOptions{
		Filter: string(VPCListFilter),
	}))
	if err != nil {
		return fmt.Errorf("could not list VPCs: %v", err)
	}

	if len(vpcs) > 0 {
		klog.Info("Will delete vpc:\n")
		for _, vpc := range vpcs {
			klog.Infof("  Label: %s, ID: %d\n", vpc.Label, vpc.ID)
		}
	}

	nodeBal, err := values.Linode.Client.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(ListFilter)))
	if err != nil {
		return err
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
		if err := values.Linode.Client.DeleteInstance(ctx, instance.ID); err != nil {
			return fmt.Errorf("could not delete instance %s: %v", instance.Label, err)
		}
		klog.Infof("  Deleted Instance %s\n", instance.Label)
	}

	if len(nodeBal) == 1 {
		if err := values.Linode.Client.DeleteNodeBalancer(ctx, nodeBal[0].ID); err != nil {
			return fmt.Errorf("could not delete nodebalancer %s: %v", *nodeBal[0].Label, err)
		}
		klog.Infof("  Deleted NodeBalancer %s\n", *nodeBal[0].Label)
	}

	if len(vpcs) == 1 {
		if err := values.Linode.Client.DeleteVPC(ctx, vpcs[0].ID); err != nil {
			return fmt.Errorf("could not delete vpc %s: %v", *nodeBal[0].Label, err)
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

func (CAPL) UpdateManifests(ctx context.Context, manifests []string, values providers.Values) error {
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
				Port: int32(values.Linode.NodeBalancerConfig.Port),
			}
			LinodeCluster.Spec.Network = v1alpha2.NetworkSpec{
				LoadBalancerType:              "NodeBalancer",
				ApiserverLoadBalancerPort:     6443,
				NodeBalancerID:                &values.Linode.NodeBalancer.ID,
				ApiserverNodeBalancerConfigID: &values.Linode.NodeBalancerConfig.ID,
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
