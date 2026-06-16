package main

import (
	"flag"

	"github.com/salvador-arreola/kubectl-ctx-diff/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // enable cloud provider auth (GKE, EKS, AKS)
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")  //nolint:errcheck
	flag.Set("alsologtostderr", "false") //nolint:errcheck
	klog.SetOutput(nil) // discard all klog output
	cmd.Execute()
}
