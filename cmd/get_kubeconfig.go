package cmd

import (
	Linode "capi-bootstrap/providers/infrastructure/linode"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"k8s.io/klog/v2"

	"github.com/helloyi/go-sshclient"
	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
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
	kubeconfigCmd.Flags().BoolP("ssh", "", false,
		"ssh directly into the node and grab the kubeconfig")
	kubeconfigCmd.Flags().StringP("identity-file", "i", "",
		"identity file to use with the ssh connection")
	kubeconfigCmd.Flags().StringP("username", "u", "root",
		"user to use with the ssh connection")
	kubeconfigCmd.Flags().StringP("password", "p", "",
		"password to use with the ssh connection")
	kubeconfigCmd.Flags().IntP("port", "", 22,
		"port to use with the ssh connection")
	getCmd.AddCommand(kubeconfigCmd)
}

func runGetKubeconfig(cmd *cobra.Command, clusterName string) error {
	var kconf string
	var err error
	if cmd.Flags().Changed("ssh") {
		kconf, err = getKubeconfigDirect(cmd, clusterName)
	}

	if err != nil {
		return err
	}

	fmt.Println(kconf)
	return nil
}

func getKubeconfigDirect(cmd *cobra.Command, clusterName string) (string, error) {
	linodeToken := os.Getenv("LINODE_TOKEN")

	if linodeToken == "" {
		return "", errors.New("linode_token is required")
	}

	linClient := Linode.NewClient(linodeToken, cmd.Context())
	instanceListFilter, err := json.Marshal(map[string]string{"tags": clusterName})
	if err != nil {
		return "", err
	}
	instances, err := linClient.ListInstances(cmd.Context(), ptr.To(linodego.ListOptions{
		Filter: string(instanceListFilter),
	}))
	if err != nil {
		return "", fmt.Errorf("Could not list instances: %v", err)
	}

	if len(instances) == 0 {
		return "", fmt.Errorf("Could not find a Linode instance with tag %s", clusterName)
	}

	var serverIP string
	for _, ip := range instances[0].IPv4 {
		if !ip.IsPrivate() {
			serverIP = ip.String()
		}
	}

	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return "", err
	}

	server := fmt.Sprintf("%s:%d", serverIP, port)

	idfile, err := cmd.Flags().GetString("identity-file")
	if err != nil {
		return "", err
	}

	idfile, err = homedir(idfile)
	if err != nil {
		return "", err
	}

	username, err := cmd.Flags().GetString("username")
	if err != nil {
		return "", err
	}

	password, err := cmd.Flags().GetString("password")
	if err != nil {
		return "", err
	}

	// build an ssh client with the right connection params
	var sClient *sshclient.Client
	defer func() {
		if sClient == nil {
			return
		}
		err := sClient.Close()
		if err != nil {
			klog.Errorf("Error closing ssh connection: %v", err)
		}
	}()

	if !cmd.Flags().Changed("identity-file") &&
		!cmd.Flags().Changed("username") &&
		!cmd.Flags().Changed("password") {

		// no args changed, default to root with ~/.ssh/id_rsa
		idfile, err = homedir(filepath.Join("~", ".ssh", "id_rsa"))
		if err != nil {
			return "", err
		}

		klog.Infof("Connecting by SSH to %s using identify file %s and username %s", server, idfile, username)
		sClient, err = sshclient.DialWithKey(server, username, idfile)
		if err != nil {
			return "", err
		}
	} else if cmd.Flags().Changed("identity-file") {
		// a key was passed, need to decide if we need to dial with a password
		if cmd.Flags().Changed("password") {
			klog.Infof("Connecting by SSH to %s using identify file %s with username %s and a password", server, idfile, username)
			sClient, err = sshclient.DialWithKeyWithPassphrase(server, username, idfile, password)
			if err != nil {
				return "", err
			}
		} else {
			klog.Infof("Connecting by SSH to %s using identify file %s and username %s", server, idfile, username)
			sClient, err = sshclient.DialWithKey(server, username, idfile)
			if err != nil {
				return "", err
			}
		}
	} else if cmd.Flags().Changed("password") {
		// a password was added and no key was passed so connect with un/pass
		klog.Infof("Connecting by SSH to %s using username %s with a password", server, username)
		sClient, err = sshclient.DialWithPasswd(server, username, password)
		if err != nil {
			return "", err
		}
	}

	// TODO switch on cluster distro
	return getKubeconfigK3s(sClient, clusterName)
}

func getKubeconfigK3s(session *sshclient.Client, clusterName string) (string, error) {
	output, err := session.Cmd(fmt.Sprintf("k3s kubectl get secret %s-kubeconfig -ojsonpath='{.data.value}'", clusterName)).Output()
	if err != nil {
		return "", err
	}

	kubeconfig, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		return "", err
	}
	return string(kubeconfig), nil
}

func homedir(filename string) (string, error) {
	home := "~/"
	if runtime.GOOS == "windows" {
		home = "~\\"
	}

	if strings.Contains(filename, home) {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		filename = strings.Replace(filename, home, "", 1)
		filename = filepath.Join(homedir, filename)
	}

	return filename, nil
}
