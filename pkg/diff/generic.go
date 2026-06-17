package diff

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

const (
	StatusEqual    = "equal"
	StatusOnlyIn1  = "only-in-1"
	StatusOnlyIn2  = "only-in-2"
	StatusModified = "modified"
)

type KeyDiff struct {
	Key      string
	Value1   string
	Value2   string
	Status   string
	Redacted bool
}

type DiffResult struct {
	Kind       string
	Name       string
	Namespace1 string
	Namespace2 string
	Keys       []KeyDiff
}

// flatVal holds a flattened value and whether it should be redacted in output.
type flatVal struct {
	value    string
	redacted bool
}

// metadata paths that are cluster-assigned or noisy - skip from diffs.
var skipPaths = map[string]bool{
	"status":                     true,
	"apiVersion":                 true,
	"kind":                       true,
	"metadata.resourceVersion":   true,
	"metadata.uid":               true,
	"metadata.creationTimestamp": true,
	"metadata.generation":        true,
	"metadata.managedFields":     true,
	"metadata.selfLink":          true,
	"spec.clusterIP":             true,
}

// skipPrefixes skips any path that starts with these (for list fields like spec.clusterIPs[0]).
var skipPrefixes = []string{
	"spec.clusterIPs",
}

// excludedByDefault are resources skipped when no explicit --filter is set.
// Users can opt in by naming them explicitly: --filter pods
var excludedByDefault = map[string]bool{
	"events":         true,
	"pods":           true,
	"replicasets":    true,
	"endpointslices": true,
	"endpoints":      true,
}

// AllResources discovers all namespaced resources via discovery API and diffs them.
// Partial discovery errors (e.g. unavailable CRD groups) are ignored; available
// resources are still diffed. Resources that fail to list are skipped silently.
// When filterNames is empty, excludedByDefault resources are skipped; naming one
// explicitly (e.g. --filter pods) bypasses the exclusion for that resource.
func AllResources(ctx context.Context, dyn1, dyn2 dynamic.Interface, disc discovery.DiscoveryInterface, ns1, ns2 string, filterNames []string) ([]DiffResult, error) {
	filter := buildFilter(filterNames)
	gvrs, kindMap, shortNames, err := discoverResources(disc)
	if err != nil && len(gvrs) == 0 {
		return nil, fmt.Errorf("discover resources: %w", err)
	}

	var results []DiffResult
	for _, gvr := range gvrs {
		kind := kindMap[gvr.String()]
		sns := shortNames[gvr.String()]
		if len(filterNames) == 0 && excludedByDefault[gvr.Resource] {
			continue
		}
		if !wantResource(filter, gvr.Resource, kind, sns) {
			continue
		}
		res, listErr := diffGVR(ctx, dyn1, dyn2, gvr, kind, ns1, ns2)
		if listErr != nil {
			continue
		}
		results = append(results, res...)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Kind != results[j].Kind {
			return results[i].Kind < results[j].Kind
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func buildFilter(names []string) func(string) bool {
	if len(names) == 0 {
		return func(string) bool { return true }
	}
	set := make(map[string]bool, len(names))
	for _, v := range names {
		set[strings.ToLower(v)] = true
	}
	return func(s string) bool { return set[strings.ToLower(s)] }
}

// wantResource reports whether a resource should be included given the filter.
// Accepts plural name (configmaps), singular kind (configmap), or short names (cm, deploy).
func wantResource(filter func(string) bool, resource, kind string, shortNames []string) bool {
	if filter(resource) || filter(strings.ToLower(kind)) {
		return true
	}
	for _, sn := range shortNames {
		if filter(strings.ToLower(sn)) {
			return true
		}
	}
	return false
}

func discoverResources(disc discovery.DiscoveryInterface) ([]schema.GroupVersionResource, map[string]string, map[string][]string, error) {
	// partial errors are common (e.g. metrics.k8s.io); carry on with what we got
	lists, err := disc.ServerPreferredNamespacedResources()

	var gvrs []schema.GroupVersionResource
	kindMap := make(map[string]string)
	shortNameMap := make(map[string][]string)

	for _, list := range lists {
		gv, parseErr := schema.ParseGroupVersion(list.GroupVersion)
		if parseErr != nil {
			continue
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") { // skip subresources
				continue
			}
			if !hasVerb(r.Verbs, "list") {
				continue
			}
			gvr := gv.WithResource(r.Name)
			gvrs = append(gvrs, gvr)
			kindMap[gvr.String()] = r.Kind
			if len(r.ShortNames) > 0 {
				shortNameMap[gvr.String()] = r.ShortNames
			}
		}
	}

	return gvrs, kindMap, shortNameMap, err
}

func hasVerb(verbs metav1.Verbs, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}

func diffGVR(ctx context.Context, dyn1, dyn2 dynamic.Interface, gvr schema.GroupVersionResource, kind, ns1, ns2 string) ([]DiffResult, error) {
	list1, err := dyn1.Resource(gvr).Namespace(ns1).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list2, err := dyn2.Resource(gvr).Namespace(ns2).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	isSecret := kind == "Secret"

	index2 := make(map[string]map[string]interface{}, len(list2.Items))
	for i := range list2.Items {
		o := list2.Items[i]
		index2[o.GetName()] = o.Object
	}

	var results []DiffResult
	for i := range list1.Items {
		o := list1.Items[i]
		name := o.GetName()
		flat1 := flattenObject(o.Object, isSecret)
		flat2 := flattenObject(index2[name], isSecret)
		results = append(results, diffFlat(kind, name, ns1, ns2, flat1, flat2))
		delete(index2, name)
	}
	for name, raw := range index2 {
		flat2 := flattenObject(raw, isSecret)
		results = append(results, diffFlat(kind, name, ns1, ns2, nil, flat2))
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

func diffFlat(kind, name, ns1, ns2 string, m1, m2 map[string]flatVal) DiffResult {
	seen := make(map[string]bool)
	var keys []KeyDiff

	for k, fv1 := range m1 {
		seen[k] = true
		fv2, exists := m2[k]
		redacted := fv1.redacted || (exists && fv2.redacted)
		switch {
		case !exists:
			v1 := fv1.value
			if redacted {
				v1 = ""
			}
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Status: StatusOnlyIn1, Redacted: redacted})
		case fv1.value == fv2.value:
			v1, v2 := fv1.value, fv2.value
			if redacted {
				v1, v2 = "", ""
			}
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Value2: v2, Status: StatusEqual, Redacted: redacted})
		default:
			v1, v2 := fv1.value, fv2.value
			if redacted {
				v1, v2 = "", ""
			}
			keys = append(keys, KeyDiff{Key: k, Value1: v1, Value2: v2, Status: StatusModified, Redacted: redacted})
		}
	}

	for k, fv2 := range m2 {
		if !seen[k] {
			v2 := fv2.value
			if fv2.redacted {
				v2 = ""
			}
			keys = append(keys, KeyDiff{Key: k, Value2: v2, Status: StatusOnlyIn2, Redacted: fv2.redacted})
		}
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i].Key < keys[j].Key })
	return DiffResult{Kind: kind, Name: name, Namespace1: ns1, Namespace2: ns2, Keys: keys}
}

func flattenObject(obj map[string]interface{}, isSecret bool) map[string]flatVal {
	if obj == nil {
		return nil
	}
	out := make(map[string]flatVal)
	flattenMap("", obj, isSecret, out)
	return out
}

func flattenMap(prefix string, m map[string]interface{}, isSecret bool, out map[string]flatVal) {
	for k, v := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if skipPaths[path] {
			continue
		}
		// suppress last-applied-configuration; it's just a serialized copy of the spec
		if prefix == "metadata.annotations" && k == "kubectl.kubernetes.io/last-applied-configuration" {
			continue
		}
		flattenValue(path, v, isSecret, out)
	}
}

func flattenValue(path string, v interface{}, isSecret bool, out map[string]flatVal) {
	for _, pfx := range skipPrefixes {
		if strings.HasPrefix(path, pfx) {
			return
		}
	}
	// secret data values: hash for change detection, never reveal content
	if isSecret && (strings.HasPrefix(path, "data.") || strings.HasPrefix(path, "stringData.")) {
		if s, ok := v.(string); ok {
			h := sha256.Sum256([]byte(s))
			out[path] = flatVal{value: fmt.Sprintf("%x", h[:]), redacted: true}
			return
		}
	}

	switch val := v.(type) {
	case map[string]interface{}:
		flattenMap(path, val, isSecret, out)
	case []interface{}:
		for i, item := range val {
			flattenValue(fmt.Sprintf("%s[%d]", path, i), item, isSecret, out)
		}
	case nil:
		// omit nil values
	default:
		out[path] = flatVal{value: fmt.Sprintf("%v", val)}
	}
}
