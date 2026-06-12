package diff

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeSecret(name, ns string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Data:       data,
	}
}

func TestSecrets_NeverExposesValues(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{"password": []byte("secret1")}))
	c2 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{"password": []byte("secret2")}))

	results, err := Secrets(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	k := results[0].Keys[0]
	if k.Value1 != "" || k.Value2 != "" {
		t.Errorf("expected empty values, got %q / %q", k.Value1, k.Value2)
	}
	if !k.Redacted {
		t.Error("expected Redacted=true")
	}
	if k.Status != StatusModified {
		t.Errorf("want modified, got %s", k.Status)
	}
}

func TestSecrets_EqualValues(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{"key": []byte("same")}))
	c2 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{"key": []byte("same")}))

	results, err := Secrets(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Keys[0].Status != StatusEqual {
		t.Errorf("want equal, got %s", results[0].Keys[0].Status)
	}
}

func TestSecrets_OnlyIn1(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{"key": []byte("val")}))
	c2 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{}))

	results, err := Secrets(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Keys[0].Status != StatusOnlyIn1 {
		t.Errorf("want only-in-1, got %s", results[0].Keys[0].Status)
	}
}

func TestSecrets_Kind(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{}))
	c2 := fake.NewSimpleClientset(makeSecret("db", "default", map[string][]byte{}))

	results, err := Secrets(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Kind != "Secret" {
		t.Errorf("want Kind=Secret, got %s", results[0].Kind)
	}
}
