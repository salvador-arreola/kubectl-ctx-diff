package diff

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentResources diffs CPU/memory requests and limits per container.
func DeploymentResources(ctx context.Context, client1, client2 kubernetes.Interface, namespace1, namespace2 string) ([]DiffResult, error) {
	return diffDeployments(ctx, client1, client2, namespace1, namespace2, extractResources)
}

// DeploymentEnvVars diffs env vars per container (literal values only; valueFrom refs skipped).
func DeploymentEnvVars(ctx context.Context, client1, client2 kubernetes.Interface, namespace1, namespace2 string) ([]DiffResult, error) {
	return diffDeployments(ctx, client1, client2, namespace1, namespace2, extractEnvVars)
}

type containerExtractor func([]corev1.Container) map[string]string

func diffDeployments(ctx context.Context, client1, client2 kubernetes.Interface, namespace1, namespace2 string, extract containerExtractor) ([]DiffResult, error) {
	list1, err := client1.AppsV1().Deployments(namespace1).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list2, err := client2.AppsV1().Deployments(namespace2).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	index2 := make(map[string]map[string]string, len(list2.Items))
	for _, d := range list2.Items {
		index2[d.Name] = extract(d.Spec.Template.Spec.Containers)
	}

	var results []DiffResult

	for _, d := range list1.Items {
		data1 := extract(d.Spec.Template.Spec.Containers)
		results = append(results, diffData("Deployment", d.Name, namespace1, namespace2, data1, index2[d.Name]))
		delete(index2, d.Name)
	}

	for name, data := range index2 {
		results = append(results, diffData("Deployment", name, namespace1, namespace2, nil, data))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func extractResources(containers []corev1.Container) map[string]string {
	m := make(map[string]string)
	for _, c := range containers {
		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			m[fmt.Sprintf("%s.requests.cpu", c.Name)] = q.String()
		}
		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			m[fmt.Sprintf("%s.requests.memory", c.Name)] = q.String()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			m[fmt.Sprintf("%s.limits.cpu", c.Name)] = q.String()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			m[fmt.Sprintf("%s.limits.memory", c.Name)] = q.String()
		}
	}
	return m
}

func extractEnvVars(containers []corev1.Container) map[string]string {
	m := make(map[string]string)
	for _, c := range containers {
		for _, e := range c.Env {
			if e.Value != "" { // skip valueFrom refs — can't compare them meaningfully
				m[fmt.Sprintf("%s.%s", c.Name, e.Name)] = e.Value
			}
		}
	}
	return m
}
