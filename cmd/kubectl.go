package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	kubectlCmd "k8s.io/kubectl/pkg/cmd"
)

var (
	DefaultKubeconfig = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
	KubeconfigEnv     = os.Getenv("KUBECONFIG")
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano())) //nolint: gosec

	kubeconfig := KubeconfigEnv
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--kubeconfig=") {
			kubeconfig = strings.Split(arg, "=")[1]
		} else if strings.HasPrefix(arg, "--kubeconfig") && i+1 < len(os.Args) {
			kubeconfig = os.Args[i+1]
		}
	}
	if kubeconfig == "" {
		kubeconfig = DefaultKubeconfig
		if _, err := os.Stat(kubeconfig); err == nil {
			os.Setenv("KUBECONFIG", kubeconfig)
		}
	}

	rootCmd.AddCommand(kubectlCmd.NewDefaultKubectlCommand())
}
