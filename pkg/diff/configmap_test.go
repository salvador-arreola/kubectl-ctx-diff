package diff

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeCM(name, ns string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Data:       data,
	}
}

func TestConfigMaps_Equal(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "val"}))
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "val"}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Keys[0].Status != StatusEqual {
		t.Errorf("want equal, got %s", results[0].Keys[0].Status)
	}
}

func TestConfigMaps_Modified(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "old"}))
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "new"}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	k := results[0].Keys[0]
	if k.Status != StatusModified {
		t.Errorf("want modified, got %s", k.Status)
	}
	if k.Value1 != "old" || k.Value2 != "new" {
		t.Errorf("wrong values: %q / %q", k.Value1, k.Value2)
	}
}

func TestConfigMaps_OnlyIn1(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "val"}))
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Keys[0].Status != StatusOnlyIn1 {
		t.Errorf("want only-in-1, got %s", results[0].Keys[0].Status)
	}
}

func TestConfigMaps_OnlyIn2(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{}))
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "val"}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Keys[0].Status != StatusOnlyIn2 {
		t.Errorf("want only-in-2, got %s", results[0].Keys[0].Status)
	}
}

func TestConfigMaps_CMOnlyIn2(t *testing.T) {
	c1 := fake.NewSimpleClientset()
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{"key": "val"}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Name != "app" {
		t.Errorf("unexpected name: %s", results[0].Name)
	}
	if results[0].Keys[0].Status != StatusOnlyIn2 {
		t.Errorf("want only-in-2, got %s", results[0].Keys[0].Status)
	}
}

func TestConfigMaps_MultipleKeys(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{
		"a": "1", "b": "2", "c": "3",
	}))
	c2 := fake.NewSimpleClientset(makeCM("app", "default", map[string]string{
		"a": "1", "b": "changed", "d": "4",
	}))

	results, err := ConfigMaps(context.Background(), c1, c2, "default")
	if err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]KeyDiff)
	for _, kd := range results[0].Keys {
		byKey[kd.Key] = kd
	}
	if byKey["a"].Status != StatusEqual {
		t.Errorf("a: want equal, got %s", byKey["a"].Status)
	}
	if byKey["b"].Status != StatusModified {
		t.Errorf("b: want modified, got %s", byKey["b"].Status)
	}
	if byKey["c"].Status != StatusOnlyIn1 {
		t.Errorf("c: want only-in-1, got %s", byKey["c"].Status)
	}
	if byKey["d"].Status != StatusOnlyIn2 {
		t.Errorf("d: want only-in-2, got %s", byKey["d"].Status)
	}
}
