package diff

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeDeployment(name, ns string, containers []corev1.Container) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: containers},
			},
		},
	}
}

func makeContainer(name string, cpuReq, memReq, cpuLim, memLim string, envVars map[string]string) corev1.Container {
	c := corev1.Container{Name: name}
	if cpuReq != "" || memReq != "" {
		c.Resources.Requests = corev1.ResourceList{}
		if cpuReq != "" {
			c.Resources.Requests[corev1.ResourceCPU] = resource.MustParse(cpuReq)
		}
		if memReq != "" {
			c.Resources.Requests[corev1.ResourceMemory] = resource.MustParse(memReq)
		}
	}
	if cpuLim != "" || memLim != "" {
		c.Resources.Limits = corev1.ResourceList{}
		if cpuLim != "" {
			c.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLim)
		}
		if memLim != "" {
			c.Resources.Limits[corev1.ResourceMemory] = resource.MustParse(memLim)
		}
	}
	for k, v := range envVars {
		c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: v})
	}
	return c
}

func TestDeploymentResources_Modified(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "100m", "128Mi", "200m", "256Mi", nil),
	}))
	c2 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "200m", "128Mi", "200m", "256Mi", nil),
	}))

	results, err := DeploymentResources(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]KeyDiff)
	for _, k := range results[0].Keys {
		byKey[k.Key] = k
	}
	if byKey["app.requests.cpu"].Status != StatusModified {
		t.Errorf("app.requests.cpu: want modified, got %s", byKey["app.requests.cpu"].Status)
	}
	if byKey["app.requests.memory"].Status != StatusEqual {
		t.Errorf("app.requests.memory: want equal, got %s", byKey["app.requests.memory"].Status)
	}
}

func TestDeploymentEnvVars_Modified(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "", "", "", "", map[string]string{"DB_HOST": "postgres-staging", "PORT": "5432"}),
	}))
	c2 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "", "", "", "", map[string]string{"DB_HOST": "postgres-prod", "PORT": "5432"}),
	}))

	results, err := DeploymentEnvVars(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]KeyDiff)
	for _, k := range results[0].Keys {
		byKey[k.Key] = k
	}
	if byKey["app.DB_HOST"].Status != StatusModified {
		t.Errorf("app.DB_HOST: want modified, got %s", byKey["app.DB_HOST"].Status)
	}
	if byKey["app.PORT"].Status != StatusEqual {
		t.Errorf("app.PORT: want equal, got %s", byKey["app.PORT"].Status)
	}
}

func TestDeploymentResources_Kind(t *testing.T) {
	c1 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "100m", "", "", "", nil),
	}))
	c2 := fake.NewSimpleClientset(makeDeployment("api", "default", []corev1.Container{
		makeContainer("app", "100m", "", "", "", nil),
	}))

	results, err := DeploymentResources(context.Background(), c1, c2, "default", "default")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Kind != "Deployment" {
		t.Errorf("want Kind=Deployment, got %s", results[0].Kind)
	}
}
