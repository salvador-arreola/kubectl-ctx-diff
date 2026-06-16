package client

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func loadingRules(kubeconfig string) *clientcmd.ClientConfigLoadingRules {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	return rules
}

func restConfig(kubeconfig, contextName string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules(kubeconfig), overrides,
	).ClientConfig()
}

// ResolveContextName returns the effective context name and validates it exists.
// Empty name resolves to current-context.
func ResolveContextName(kubeconfig, name string) (string, error) {
	cfg, err := loadingRules(kubeconfig).Load()
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

// ValidateNamespace returns an error if namespace does not exist on the cluster.
func ValidateNamespace(ctx context.Context, cs kubernetes.Interface, namespace string) error {
	_, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("namespace %q not found", namespace)
		}
		return fmt.Errorf("namespace %q: %w", namespace, err)
	}
	return nil
}

// New builds a typed clientset for the named kubeconfig context.
func New(kubeconfig, contextName string) (kubernetes.Interface, error) {
	cfg, err := restConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// NewDynamic builds a dynamic client for the named kubeconfig context.
func NewDynamic(kubeconfig, contextName string) (dynamic.Interface, error) {
	cfg, err := restConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(cfg)
}

// NewDiscovery builds a discovery client for the named kubeconfig context.
func NewDiscovery(kubeconfig, contextName string) (discovery.DiscoveryInterface, error) {
	cfg, err := restConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClientForConfig(cfg)
}
