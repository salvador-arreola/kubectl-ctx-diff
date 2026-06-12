package diff

import (
	"context"
	"crypto/sha256"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Secrets diffs Secret keys (never values) between two contexts.
func Secrets(ctx context.Context, client1, client2 kubernetes.Interface, namespace1, namespace2 string) ([]DiffResult, error) {
	list1, err := client1.CoreV1().Secrets(namespace1).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list2, err := client2.CoreV1().Secrets(namespace2).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	index2 := make(map[string]map[string][]byte, len(list2.Items))
	for _, s := range list2.Items {
		index2[s.Name] = s.Data
	}

	var results []DiffResult

	for _, s := range list1.Items {
		results = append(results, diffSecretData(s.Name, namespace1, namespace2, s.Data, index2[s.Name]))
		delete(index2, s.Name)
	}

	for name, data := range index2 {
		results = append(results, diffSecretData(name, namespace1, namespace2, nil, data))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func diffSecretData(name, namespace1, namespace2 string, data1, data2 map[string][]byte) DiffResult {
	seen := make(map[string]bool)
	var keys []KeyDiff

	for k, v1 := range data1 {
		seen[k] = true
		v2, exists := data2[k]
		var status string
		switch {
		case !exists:
			status = StatusOnlyIn1
		case sha256.Sum256(v1) == sha256.Sum256(v2):
			status = StatusEqual
		default:
			status = StatusModified
		}
		keys = append(keys, KeyDiff{Key: k, Status: status, Redacted: true})
	}

	for k := range data2 {
		if !seen[k] {
			keys = append(keys, KeyDiff{Key: k, Status: StatusOnlyIn2, Redacted: true})
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Key < keys[j].Key
	})

	return DiffResult{Kind: "Secret", Name: name, Namespace1: namespace1, Namespace2: namespace2, Keys: keys}
}
