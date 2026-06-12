package main

import (
	"github.com/salvador-arreola/kubectl-ctx-diff/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // enable cloud provider auth (GKE, EKS, AKS)
)

func main() {
	cmd.Execute()
}
