package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	k8snet "k8s.io/utils/net"
	"sigs.k8s.io/cluster-api/api/v1beta1"

	"capi-bootstrap/types"
)

func BuildNodeInfoList(ctx context.Context, kubeconfig []byte) ([]*types.NodeInfo, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	client := kubernetes.NewForConfigOrDie(config)

	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodeInfoList := make([]*types.NodeInfo, 0, len(nodeList.Items))

	for _, node := range nodeList.Items {
		var status string

		for _, cond := range node.Status.Conditions {
			if cond.Reason == "KubeletReady" {
				if cond.Status == "True" {
					status = string(v1beta1.ReadyCondition)
				} else {
					status = "NotReady"
				}
				break
			}
		}

		var extIP string
		for _, addr := range node.Status.Addresses {
			if addr.Type == "ExternalIP" {
				ip := net.ParseIP(addr.Address)
				if k8snet.IsIPv4(ip) {
					extIP = addr.Address
					break
				}
				continue
			}
		}

		var timestamp string
		switch {
		case time.Since(node.GetCreationTimestamp().Local()).Hours()/24 < 1:
			timestamp = fmt.Sprintf("%.1fh", time.Since(node.GetCreationTimestamp().Local()).Hours())
		case time.Since(node.GetCreationTimestamp().Local()).Hours()/24 > 1:
			timestamp = fmt.Sprintf("%.2fd", time.Since(node.GetCreationTimestamp().Local()).Hours()/24)
		case time.Since(node.GetCreationTimestamp().Local()).Hours() < 1:
			timestamp = fmt.Sprintf("%fm", time.Since(node.GetCreationTimestamp().Local()).Minutes())
		}
		nodeInfoList = append(nodeInfoList, &types.NodeInfo{
			Name:              node.Name,
			Status:            status,
			Version:           node.Status.NodeInfo.KubeletVersion,
			ExternalIP:        extIP,
			DaysSinceCreation: timestamp,
		})
	}
	return nodeInfoList, nil
}

func TabWriteClusters(w io.Writer, clusters []types.ClusterInfo) error {
	for _, cluster := range clusters {
		fmt.Printf("Cluster: %s\n", cluster.Name)

		numBytes, err := fmt.Fprintln(w, "Name\tStatus\tVersion\tExternal IP\tAge")
		if err != nil || numBytes == 0 {
			return err
		}
		for _, node := range cluster.Nodes {
			if node == nil {
				continue
			}
			numBytes, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", node.Name, node.Status,
				node.Version, node.ExternalIP, node.DaysSinceCreation)
			if err != nil || numBytes == 0 {
				return err
			}
		}
	}
	return nil
}
