package docker

import (
	"context"
	"os"
	"testing"
	"time"

	parser "kuberMendez/deployment-parser"
)

type dockerRunInput struct {
	DeploymentName string
	Replicas       int
	Spec           []parser.Container
}

func TestDockerContainerRun(t *testing.T) {
	requireDockerIntegration(t)

	tests := []struct {
		name    string
		input   dockerRunInput
		wantErr bool
	}{
		{
			name: "test valid container creation",
			input: dockerRunInput{
				DeploymentName: "test-deployment",
				Replicas:       3,
				Spec: []parser.Container{
					{
						Name:  "nginx",
						Image: "nginx",
						Ports: []parser.Port{
							{
								ContainerPort: 80,
								HostPort:      true,
							},
						},
					},
					{
						Name:  "rabbitmq",
						Image: "rabbitmq",
						Ports: []parser.Port{
							{
								ContainerPort: 80,
								HostPort:      true,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test bad image container creation",
			input: dockerRunInput{
				DeploymentName: "test-deployment",
				Spec: []parser.Container{
					{
						Name:  "nginx",
						Image: "fakeimagfsd",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		runtime, err := NewRuntime()
		if err != nil {
			t.Fatalf("create Docker runtime: %v", err)
		}
		t.Run(test.name, func(t *testing.T) {

			for _, container := range test.input.Spec {
				err := runtime.ContainerRun(context.TODO(), container, test.input.DeploymentName, test.input.Replicas)

				if test.wantErr && err == nil {
					t.Fatal("Docker returned nil error, want error")
				}

				if !test.wantErr && err != nil {
					t.Fatalf("Docker returned error, want nil: %v", err)
				}

			}
		})
		time.Sleep(5 * time.Second)
		if err := runtime.RemoveContainers(context.TODO(), "test-deployment"); err != nil {
			t.Errorf("remove test containers: %v", err)
		}
		if err := runtime.Close(); err != nil {
			t.Errorf("close Docker runtime: %v", err)
		}
	}
}

func TestListContainers(t *testing.T) {
	requireDockerIntegration(t)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "list deployments test",
			input:   "Nico",
			wantErr: false,
		},
	}

	for _, test := range tests {
		runtime, err := NewRuntime()
		if err != nil {
			t.Fatalf("create Docker runtime: %v", err)
		}
		t.Run(test.name, func(t *testing.T) {
			_, err := runtime.ListContainers(context.TODO(), test.input)

			if test.wantErr && err == nil {
				t.Fatal("ContainerList returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("ContainerList returned error, want nil: %v", err)
			}

			if err := runtime.RemoveContainers(context.TODO(), "test-deployment"); err != nil {
				t.Errorf("remove test containers: %v", err)
			}
		})
		if err := runtime.Close(); err != nil {
			t.Errorf("close Docker runtime: %v", err)
		}
	}
}

func requireDockerIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("KUBERMENDEZ_DOCKER_INTEGRATION") == "" {
		t.Skip("set KUBERMENDEZ_DOCKER_INTEGRATION=1 to run Docker integration tests")
	}
}
