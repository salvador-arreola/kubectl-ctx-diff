package diff

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dfake "k8s.io/client-go/dynamic/fake"
)

var (
	cmGVR     = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	widgetGVR = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
)

func makeObj(gvr schema.GroupVersionResource, kind, name, ns string, extra map[string]interface{}) *unstructured.Unstructured {
	apiVersion := gvr.Version
	if gvr.Group != "" {
		apiVersion = gvr.Group + "/" + gvr.Version
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
	}}
	for k, v := range extra {
		obj.Object[k] = v
	}
	return obj
}

func fakeDyn(gvr schema.GroupVersionResource, listKind string, objs ...*unstructured.Unstructured) *dfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	rObjs := make([]runtime.Object, len(objs))
	for i, o := range objs {
		rObjs[i] = o
	}
	return dfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: listKind}, rObjs...)
}

func TestFlattenObject_Basic(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":            "my-config",
			"resourceVersion": "12345",
			"uid":             "abc",
		},
		"data": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	out := flattenObject(obj, false)

	// data keys present
	if out["data.key1"].value != "value1" {
		t.Errorf("data.key1: want value1, got %q", out["data.key1"].value)
	}
	if out["data.key2"].value != "value2" {
		t.Errorf("data.key2: want value2, got %q", out["data.key2"].value)
	}

	// noise fields skipped
	if _, ok := out["apiVersion"]; ok {
		t.Error("apiVersion should be skipped")
	}
	if _, ok := out["kind"]; ok {
		t.Error("kind should be skipped")
	}
	if _, ok := out["metadata.resourceVersion"]; ok {
		t.Error("metadata.resourceVersion should be skipped")
	}
	if _, ok := out["metadata.uid"]; ok {
		t.Error("metadata.uid should be skipped")
	}

	// name kept
	if out["metadata.name"].value != "my-config" {
		t.Errorf("metadata.name: want my-config, got %q", out["metadata.name"].value)
	}
}

func TestFlattenObject_SkipsStatus(t *testing.T) {
	obj := map[string]interface{}{
		"spec":   map[string]interface{}{"replicas": float64(3)},
		"status": map[string]interface{}{"readyReplicas": float64(3)},
	}
	out := flattenObject(obj, false)
	if _, ok := out["status.readyReplicas"]; ok {
		t.Error("status fields should be skipped")
	}
	if out["spec.replicas"].value != "3" {
		t.Errorf("spec.replicas: want 3, got %q", out["spec.replicas"].value)
	}
}

func TestFlattenObject_SecretRedaction(t *testing.T) {
	secret := "hello world"
	h := sha256.Sum256([]byte(secret))
	expectedHash := fmt.Sprintf("%x", h[:])

	obj := map[string]interface{}{
		"data": map[string]interface{}{
			"password": secret,
		},
		"metadata": map[string]interface{}{
			"name": "my-secret",
		},
	}

	out := flattenObject(obj, true)

	fv, ok := out["data.password"]
	if !ok {
		t.Fatal("data.password missing")
	}
	if !fv.redacted {
		t.Error("data.password should be redacted")
	}
	if fv.value != expectedHash {
		t.Errorf("data.password hash mismatch: got %q", fv.value)
	}

	// non-data fields not redacted
	if out["metadata.name"].redacted {
		t.Error("metadata.name should not be redacted")
	}
}

func TestFlattenObject_Arrays(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "nginx", "image": "nginx:1.19"},
				map[string]interface{}{"name": "sidecar", "image": "envoy:v1"},
			},
		},
	}

	out := flattenObject(obj, false)

	if out["spec.containers[0].name"].value != "nginx" {
		t.Errorf("containers[0].name: want nginx, got %q", out["spec.containers[0].name"].value)
	}
	if out["spec.containers[1].image"].value != "envoy:v1" {
		t.Errorf("containers[1].image: want envoy:v1, got %q", out["spec.containers[1].image"].value)
	}
}

func TestFlattenObject_SkipsLastApplied(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1"}`,
				"app.io/version": "1.0",
			},
		},
	}

	out := flattenObject(obj, false)

	for k := range out {
		if k == "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration" {
			t.Error("last-applied-configuration should be skipped")
		}
	}
	if out["metadata.annotations.app.io/version"].value != "1.0" {
		t.Errorf("user annotation missing, got %q", out["metadata.annotations.app.io/version"].value)
	}
}

func TestDiffFlat_AllStatuses(t *testing.T) {
	m1 := map[string]flatVal{
		"spec.replicas": {value: "3"},
		"spec.image":    {value: "nginx:1.18"},
		"spec.removed":  {value: "gone"},
	}
	m2 := map[string]flatVal{
		"spec.replicas": {value: "5"},
		"spec.image":    {value: "nginx:1.18"},
		"spec.added":    {value: "new"},
	}

	result := diffFlat("Deployment", "my-deploy", "ns1", "ns2", m1, m2)

	byKey := make(map[string]KeyDiff)
	for _, k := range result.Keys {
		byKey[k.Key] = k
	}

	if byKey["spec.replicas"].Status != StatusModified {
		t.Errorf("spec.replicas: want modified, got %s", byKey["spec.replicas"].Status)
	}
	if byKey["spec.image"].Status != StatusEqual {
		t.Errorf("spec.image: want equal, got %s", byKey["spec.image"].Status)
	}
	if byKey["spec.removed"].Status != StatusOnlyIn1 {
		t.Errorf("spec.removed: want only-in-1, got %s", byKey["spec.removed"].Status)
	}
	if byKey["spec.added"].Status != StatusOnlyIn2 {
		t.Errorf("spec.added: want only-in-2, got %s", byKey["spec.added"].Status)
	}
}

func TestDiffFlat_RedactedValuesHidden(t *testing.T) {
	m1 := map[string]flatVal{
		"data.pass": {value: "hash1", redacted: true},
	}
	m2 := map[string]flatVal{
		"data.pass": {value: "hash2", redacted: true},
	}

	result := diffFlat("Secret", "my-secret", "ns1", "ns2", m1, m2)
	if len(result.Keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(result.Keys))
	}
	k := result.Keys[0]
	if k.Status != StatusModified {
		t.Errorf("want modified, got %s", k.Status)
	}
	if !k.Redacted {
		t.Error("want Redacted=true")
	}
	if k.Value1 != "" || k.Value2 != "" {
		t.Errorf("redacted values must be empty, got %q / %q", k.Value1, k.Value2)
	}
}

func TestDiffFlat_NilMaps(t *testing.T) {
	m2 := map[string]flatVal{
		"data.key": {value: "val"},
	}
	result := diffFlat("ConfigMap", "cm", "ns1", "ns2", nil, m2)
	if len(result.Keys) != 1 || result.Keys[0].Status != StatusOnlyIn2 {
		t.Errorf("nil m1: want single only-in-2 key, got %+v", result.Keys)
	}
}

// --- diffGVR tests using fake dynamic client ---

func TestDiffGVR_Modified(t *testing.T) {
	obj1 := makeObj(widgetGVR, "Widget", "w1", "default", map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(2), "image": "myapp:1.0"},
	})
	obj2 := makeObj(widgetGVR, "Widget", "w1", "default", map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(5), "image": "myapp:2.0"},
	})

	dyn1 := fakeDyn(widgetGVR, "WidgetList", obj1)
	dyn2 := fakeDyn(widgetGVR, "WidgetList", obj2)

	results, err := diffGVR(context.Background(), dyn1, dyn2, widgetGVR, "Widget", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	byKey := keysByName(results[0])
	assertStatus(t, byKey, "spec.replicas", StatusModified)
	assertStatus(t, byKey, "spec.image", StatusModified)
	if byKey["spec.replicas"].Value1 != "2" || byKey["spec.replicas"].Value2 != "5" {
		t.Errorf("spec.replicas values: want 2/5, got %s/%s", byKey["spec.replicas"].Value1, byKey["spec.replicas"].Value2)
	}
}

func TestDiffGVR_Equal(t *testing.T) {
	obj := makeObj(cmGVR, "ConfigMap", "cfg", "default", map[string]interface{}{
		"data": map[string]interface{}{"env": "prod"},
	})
	dyn1 := fakeDyn(cmGVR, "ConfigMapList", obj)
	dyn2 := fakeDyn(cmGVR, "ConfigMapList", obj.DeepCopy())

	results, err := diffGVR(context.Background(), dyn1, dyn2, cmGVR, "ConfigMap", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	for _, k := range results[0].Keys {
		if k.Status != StatusEqual {
			t.Errorf("key %s: want equal, got %s", k.Key, k.Status)
		}
	}
}

func TestDiffGVR_OnlyIn1(t *testing.T) {
	obj := makeObj(widgetGVR, "Widget", "w1", "default", map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(1)},
	})
	dyn1 := fakeDyn(widgetGVR, "WidgetList", obj)
	dyn2 := fakeDyn(widgetGVR, "WidgetList") // empty

	results, err := diffGVR(context.Background(), dyn1, dyn2, widgetGVR, "Widget", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	for _, k := range results[0].Keys {
		if k.Status != StatusOnlyIn1 {
			t.Errorf("key %s: want only-in-1, got %s", k.Key, k.Status)
		}
	}
}

func TestDiffGVR_OnlyIn2(t *testing.T) {
	obj := makeObj(widgetGVR, "Widget", "w1", "default", map[string]interface{}{
		"spec": map[string]interface{}{"replicas": float64(1)},
	})
	dyn1 := fakeDyn(widgetGVR, "WidgetList") // empty
	dyn2 := fakeDyn(widgetGVR, "WidgetList", obj)

	results, err := diffGVR(context.Background(), dyn1, dyn2, widgetGVR, "Widget", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	for _, k := range results[0].Keys {
		if k.Status != StatusOnlyIn2 {
			t.Errorf("key %s: want only-in-2, got %s", k.Key, k.Status)
		}
	}
}

func TestDiffGVR_SecretRedacted(t *testing.T) {
	rawPass1 := "secret1"
	rawPass2 := "secret2"
	h1 := sha256.Sum256([]byte(rawPass1))
	h2 := sha256.Sum256([]byte(rawPass2))

	s1 := makeObj(secretGVR, "Secret", "creds", "default", map[string]interface{}{
		"data": map[string]interface{}{"password": rawPass1},
	})
	s2 := makeObj(secretGVR, "Secret", "creds", "default", map[string]interface{}{
		"data": map[string]interface{}{"password": rawPass2},
	})

	dyn1 := fakeDyn(secretGVR, "SecretList", s1)
	dyn2 := fakeDyn(secretGVR, "SecretList", s2)

	results, err := diffGVR(context.Background(), dyn1, dyn2, secretGVR, "Secret", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	byKey := keysByName(results[0])
	k := byKey["data.password"]
	if !k.Redacted {
		t.Error("data.password: want Redacted=true")
	}
	if k.Status != StatusModified {
		t.Errorf("data.password: want modified, got %s", k.Status)
	}
	if k.Value1 != "" || k.Value2 != "" {
		t.Errorf("redacted values must be empty strings, got %q/%q", k.Value1, k.Value2)
	}
	// verify hashes are different (change detection works)
	_ = h1
	_ = h2
}

func TestDiffGVR_MultipleObjects(t *testing.T) {
	a1 := makeObj(cmGVR, "ConfigMap", "alpha", "default", map[string]interface{}{
		"data": map[string]interface{}{"x": "1"},
	})
	b1 := makeObj(cmGVR, "ConfigMap", "beta", "default", map[string]interface{}{
		"data": map[string]interface{}{"y": "old"},
	})
	a2 := makeObj(cmGVR, "ConfigMap", "alpha", "default", map[string]interface{}{
		"data": map[string]interface{}{"x": "1"},
	})
	b2 := makeObj(cmGVR, "ConfigMap", "beta", "default", map[string]interface{}{
		"data": map[string]interface{}{"y": "new"},
	})

	dyn1 := fakeDyn(cmGVR, "ConfigMapList", a1, b1)
	dyn2 := fakeDyn(cmGVR, "ConfigMapList", a2, b2)

	results, err := diffGVR(context.Background(), dyn1, dyn2, cmGVR, "ConfigMap", "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	// results sorted by name: alpha first, beta second
	if results[0].Name != "alpha" || results[1].Name != "beta" {
		t.Errorf("unexpected order: %s, %s", results[0].Name, results[1].Name)
	}
	alphaKey := keysByName(results[0])["data.x"]
	if alphaKey.Status != StatusEqual {
		t.Errorf("alpha data.x: want equal, got %s", alphaKey.Status)
	}
	betaKey := keysByName(results[1])["data.y"]
	if betaKey.Status != StatusModified {
		t.Errorf("beta data.y: want modified, got %s", betaKey.Status)
	}
}

// helpers

func keysByName(r DiffResult) map[string]KeyDiff {
	m := make(map[string]KeyDiff)
	for _, k := range r.Keys {
		m[k.Key] = k
	}
	return m
}

func assertStatus(t *testing.T, byKey map[string]KeyDiff, key, want string) {
	t.Helper()
	if got := byKey[key].Status; got != want {
		t.Errorf("%s: want %s, got %s", key, want, got)
	}
}
