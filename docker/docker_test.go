package docker

import (
	parser "kuberMendez/deployment-parser"
	"testing"
)

type dockerRunInput struct {
	DeploymentName string
	Spec           parser.Container
}

func TestDockerContainerRun(t *testing.T) {
	hostPort := 8080

tests := []struct {
	name    string
	input   dockerRunInput
	wantErr bool
	}{
		{
			name: "test valid container creation",
			input: dockerRunInput{
				DeploymentName: "test-deployment",
				Spec: parser.Container{
					Name:  "nginx",
					Image: "nginx",
					Ports: []parser.Port{
						{
							ContainerPort: 80,
							HostPort:      &hostPort,
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
				Spec: parser.Container{
					Name: "nginx",
					Image: "fakeimagfsd",
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := DockerRun(test.input.Spec, test.input.DeploymentName)

			if test.wantErr && err == nil{
				t.Fatal("Docker returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("Docker returned error, want nil: %v", err)
			}
		})
	}
}

func TestListContainers(t *testing.T){
	tests := []struct{
		name string
		input string
		wantErr bool
	}{
		{
			name: "list deployments test",
			input: "Nico",
			wantErr: false,
		},

	}

	for _, test := range tests{
		t.Run(test.name, func(t *testing.T) {
			err := ListContainers(test.input)

			if test.wantErr && err == nil{
				t.Fatal("ContainerList returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("ContainerList returned error, want nil: %v", err)
			}
		})
	}
}