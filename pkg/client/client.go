package client

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ResolveContextName returns the effective context name and validates it exists.
// Empty name resolves to current-context.
func ResolveContextName(name string) (string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	if err != nil {
		return "", err
	}
	if name == "" {
		if cfg.CurrentContext == "" {
			return "", fmt.Errorf("no current-context set in kubeconfig")
		}
		return cfg.CurrentContext, nil
	}
	if _, ok := cfg.Contexts[name]; !ok {
		return "", fmt.Errorf("context %q not found in kubeconfig", name)
	}
	return name, nil
}

// New builds a clientset for the named kubeconfig context.
// Empty contextName uses the current-context.
func New(contextName string) (kubernetes.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides,
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
