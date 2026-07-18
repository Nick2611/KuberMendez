package docker

import (
	"context"
	parser "kuberMendez/deployment-parser"
	"testing"
	"time"
)

type dockerRunInput struct {
	DeploymentName string
	Replicas 	   int
	Spec           []parser.Container
}

func TestDockerContainerRun(t *testing.T) {

tests := []struct {
	name    string
	input   dockerRunInput
	wantErr bool
	}{
		{
			name: "test valid container creation",
			input: dockerRunInput{
				DeploymentName: "test-deployment",
				Replicas: 3,
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
						Name: "nginx",
						Image: "fakeimagfsd",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, container := range test.input.Spec{
				err := DockerRun(context.TODO(),container, test.input.DeploymentName, test.input.Replicas)

				if test.wantErr && err == nil{
					t.Fatal("Docker returned nil error, want error")
				}

				if !test.wantErr && err != nil {
					t.Fatalf("Docker returned error, want nil: %v", err)		
				}

			}
		})
		time.Sleep(10 * time.Second)
		RemoveContainers("test-deployment")
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
			_, err := ListContainers(context.TODO(), test.input)

			if test.wantErr && err == nil{
				t.Fatal("ContainerList returned nil error, want error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("ContainerList returned error, want nil: %v", err)
			}

			RemoveContainers("test-deployment")
		})
	}
}