package diff

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	StatusEqual    = "equal"
	StatusOnlyIn1  = "only-in-1"
	StatusOnlyIn2  = "only-in-2"
	StatusModified = "modified"
)

type KeyDiff struct {
	Key    string
	Value1 string
	Value2 string
	Status string
}

type DiffResult struct {
	Name       string
	Namespace1 string
	Namespace2 string
	Keys       []KeyDiff
}

// ConfigMaps fetches ConfigMaps from both clients in their respective namespaces and diffs them key by key.
func ConfigMaps(ctx context.Context, client1, client2 kubernetes.Interface, namespace1, namespace2 string) ([]DiffResult, error) {
	list1, err := client1.CoreV1().ConfigMaps(namespace1).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list2, err := client2.CoreV1().ConfigMaps(namespace2).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	index2 := make(map[string]map[string]string, len(list2.Items))
	for _, cm := range list2.Items {
		index2[cm.Name] = cm.Data
	}

	var results []DiffResult

	for _, cm := range list1.Items {
		results = append(results, diffData(cm.Name, namespace1, namespace2, cm.Data, index2[cm.Name]))
		delete(index2, cm.Name)
	}

	// ConfigMaps present only in context-2
	for name, data := range index2 {
		results = append(results, diffData(name, namespace1, namespace2, nil, data))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func diffData(name, namespace1, namespace2 string, data1, data2 map[string]string) DiffResult {
	seen := make(map[string]bool)
	var keys []KeyDiff

	for k, v1 := range data1 {
		seen[k] = true
		v2, exists := data2[k]
		switch {
		case !exists:
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Status: StatusOnlyIn1})
		case v1 == v2:
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Value2: v2, Status: StatusEqual})
		default:
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Value2: v2, Status: StatusModified})
		}
	}

	for k, v2 := range data2 {
		if !seen[k] {
			keys = append(keys, KeyDiff{Key: k, Value2: v2, Status: StatusOnlyIn2})
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Key < keys[j].Key
	})

	return DiffResult{Name: name, Namespace1: namespace1, Namespace2: namespace2, Keys: keys}
}
