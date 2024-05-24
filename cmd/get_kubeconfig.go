/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"capi-bootstrap/pkg"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"k8s.io/utils/ptr"
	"log"
	"net"
	"os"
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGetKubeconfig(cmd, args[0])
	},
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("please specify a cluster name")
		}
		return nil
	},
}

func init() {
	kubeconfigCmd.Flags().BoolP("direct", "", false,
		"connect directly to a cluster node to retrieve kubeconfig, requires ssh keys loaded into your ssh agent with access to the nodes")
	getCmd.AddCommand(kubeconfigCmd)

}

func runGetKubeconfig(cmd *cobra.Command, clusterName string) error {
	if cmd.Flags().Changed("direct") {
		return getKubeconfigDirect(clusterName)
	}
	return nil
}

func getKubeconfigDirect(clusterName string) error {
	ctx := context.Background()
	linodeToken := os.Getenv("LINODE_TOKEN")

	if linodeToken == "" {
		log.Fatal("linode_token is required")
	}

	linClient := pkg.LinodeClient(linodeToken, ctx)
	instanceListFilter, err := json.Marshal(map[string]string{"tags": clusterName})
	if err != nil {
		log.Fatal(err)
	}
	instances, err := linClient.ListInstances(ctx, ptr.To(linodego.ListOptions{
		Filter: string(instanceListFilter),
	}))
	if err != nil {
		log.Fatalf("Could not list instances: %v", err)
	}
	if len(instances) == 0 {
		log.Fatalf("Could not find a Linode instance with tag %s", clusterName)
	}

	var serverIP string

	for _, ip := range instances[0].IPv4 {
		if !ip.IsPrivate() {
			serverIP = ip.String()
		}
	}
	server := serverIP + ":22"

	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock == "" {
		log.Fatalf("SSH_AUTH_SOCK is not set, ensure you ahve a running ssh agent before trying to download kubeconfig directly from a machine")
	}

	// Connect to the SSH agent.
	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		log.Fatalf("failed to connect to SSH agent: %v", err)
		return err
	}
	defer conn.Close()

	// Set up the SSH client configuration.
	// Create a new SSH agent client.
	agentClient := agent.NewClient(conn)

	// Set up the SSH client configuration to use the agent for authentication.
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For testing purposes only
	}

	// Establish the SSH connection.
	client, err := ssh.Dial("tcp", server, config)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	// Create a new session.
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	kubeconfig, err := getKubeconfig(session)
	if err != nil {
		return err
	}

	// Print kubeconfig
	fmt.Println(kubeconfig)

	return nil
}

func getKubeconfig(session *ssh.Session) (string, error) {
	output, err := session.Output("k3s kubectl get secret test-k3s-kubeconfig -ojsonpath='{.data.value}'")
	if err != nil {
		log.Fatalf("failed to get kubeconfig: %v", err)
		return "", err
	}

	kubeconfig, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		log.Fatal("error:", err)
	}
	return string(kubeconfig), nil
}
