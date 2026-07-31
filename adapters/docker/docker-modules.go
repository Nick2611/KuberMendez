package docker

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
	"time"

	parser "kuberMendez/deployment-parser"
	runtimecontract "kuberMendez/runtime"

	"github.com/devjefster/GoShortUniqueID/idgen"
	"github.com/docker/docker/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	LabelCreator           = "Creator"
	LabelDeploymentName    = "DeploymentName"
	LabelContainerName     = "ContainerName"
	LabelContainerSpecHash = "ContainerSpecHash"
)

type Runtime struct {
	client *client.Client
}

func NewRuntime() (*Runtime, error) {
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	return &Runtime{client: apiClient}, nil
}

func (d *Runtime) Close() error {
	if d == nil || d.client == nil {
		return nil
	}
	return d.client.Close()
}

func (d *Runtime) ContainerRun(ctx context.Context, spec parser.Container, deploymentName string, replicas int) error {
	ctx, close := context.WithTimeout(ctx, 60*time.Second)
	defer close()

	if replicas < 0 {
		return fmt.Errorf("replicas cannot be negative: %d", replicas)
	}

	var image string = spec.Image
	var envList []string

	idGen := idgen.New(6, "", "")

	if len(spec.Env) != 0 {
		for _, env := range spec.Env {
			envList = append(envList, fmt.Sprintf("%v=%v", env.Name, env.Value))
		}
	}

	exposedPorts := make(network.PortSet)
	portBindings := make(network.PortMap)

	for _, port := range spec.Ports {
		p, err := network.ParsePort(fmt.Sprintf("%d/tcp", port.ContainerPort))
		if err != nil {
			return fmt.Errorf("Parse port %d:%q", port.ContainerPort, err)
		}
		exposedPorts[p] = struct{}{}
		portBindings[p] = []network.PortBinding{}
		hostIP, err := netip.ParseAddr("127.0.0.1")
		if err != nil {
			return fmt.Errorf("Parse address %q:%w", hostIP, err)
		}

		if port.HostPort {
			hostPort := network.PortBinding{
				HostIP:   hostIP,
				HostPort: "",
			}
			portBindings[p] = append(portBindings[p], hostPort)

		}

	}

	reader, err := d.client.ImagePull(ctx, fmt.Sprintf("docker.io/library/%v", image), client.ImagePullOptions{})
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			return fmt.Errorf("docker daemon not running: %w", err)

		} else if errdefs.IsNotFound(err) { //TODO Change deprecated method
			fmt.Println("Image not found", image)
			return fmt.Errorf("pull image %q: %w", image, err)
		}
		return err
	}
	defer reader.Close()
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("read image pull output: %w", err)
	}

	for i := 1; i <= replicas; i++ {
		resp, err := d.client.ContainerCreate(ctx, client.ContainerCreateOptions{
			Image: image,
			Name:  fmt.Sprintf("%v_%v", spec.Name, idGen.Generate()),
			Config: &container.Config{
				Labels: map[string]string{
					LabelCreator:           "Kubermendez",
					LabelDeploymentName:    deploymentName,
					LabelContainerName:     spec.Name,
					LabelContainerSpecHash: runtimecontract.ContainerSpecHash(spec),
				},
				Env:          envList,
				ExposedPorts: exposedPorts,
			},
			HostConfig: &container.HostConfig{
				PortBindings: portBindings,
			},
		})
		if err != nil {
			return fmt.Errorf("create container %q: %w", spec.Name, err)
		}

		if startResult, err := d.client.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("start container %q: %w", spec.Name, err)
		} else {
			fmt.Println(startResult)
		}

		out, err := d.client.ContainerLogs(ctx, resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return fmt.Errorf("Container lgos %q:%w", resp.ID, err)
		}

		stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	}

	return nil
}

func (d *Runtime) ListContainers(ctx context.Context, deploymentName string) ([]runtimecontract.ContainerState, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	filters := make(client.Filters)

	if deploymentName == "all" {
		filters.Add("label", fmt.Sprintf("%s=Kubermendez", LabelCreator))
	} else {
		filters.Add(
			"label",
			fmt.Sprintf("%s=%s", LabelDeploymentName, deploymentName),
		)
	}

	containers, err := d.client.ContainerList(
		ctx,
		client.ContainerListOptions{
			Filters: filters,
			All:     true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	if len(containers.Items) == 0 {
		return []runtimecontract.ContainerState{}, nil
	}

	result := make([]runtimecontract.ContainerState, 0, len(containers.Items))

	for _, container := range containers.Items {
		config, err := d.client.ContainerInspect(ctx, container.ID, client.ContainerInspectOptions{})
		if err != nil {
			return nil, fmt.Errorf("inspect container %q: %w", container.ID, err)
		}

		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}

		result = append(result, runtimecontract.ContainerState{
			ID:       container.ID,
			SpecName: container.Labels[LabelContainerName],
			SpecHash: container.Labels[LabelContainerSpecHash],
			Name:     name,
			Image:    container.Image,
			Status:   container.Status,
			Ports:    parsePorts(container.Ports),
			Env:      parseEnvVars(config.Container.Config.Env),
		})
	}

	return result, nil
}

func parsePorts(ports []container.PortSummary) []runtimecontract.PortState {
	result := make([]runtimecontract.PortState, 0, len(ports))
	for _, port := range ports {
		result = append(result, runtimecontract.PortState{
			ContainerPort: int(port.PrivatePort),
			HostPort:      port.PublicPort != 0,
		})
	}
	return result
}

func parseEnvVars(env []string) []runtimecontract.EnvVar {
	result := make([]runtimecontract.EnvVar, 0, len(env))
	for _, item := range env {
		name, value, _ := strings.Cut(item, "=")
		result = append(result, runtimecontract.EnvVar{
			Name:  name,
			Value: value,
		})
	}
	return result
}

func (d *Runtime) RemoveContainersByID(ctx context.Context, containerIDs []string) error {
	if len(containerIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	for _, containerID := range containerIDs {
		_, err := d.client.ContainerStop(ctx, containerID, client.ContainerStopOptions{})
		if err != nil {
			return fmt.Errorf("container stop %q: %w", containerID, err)
		}
		_, err = d.client.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{})
		if err != nil {
			return fmt.Errorf("container remove %q: %w", containerID, err)
		}
		fmt.Println("Container", containerID, "removed")
	}

	return nil
}

func (d *Runtime) RemoveContainers(ctx context.Context, deploymentName string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	filters := make(client.Filters)
	filters.Add("label", fmt.Sprintf("%s=%v", LabelDeploymentName, deploymentName))

	containers, err := d.client.ContainerList(ctx, client.ContainerListOptions{Filters: filters, All: true})
	if err != nil {
		return fmt.Errorf("Container list %w", err)
	}

	if len(containers.Items) == 0 {
		fmt.Println("No containers to delete")

	} else {

		for _, container := range containers.Items {
			_, err := d.client.ContainerStop(ctx, container.ID, client.ContainerStopOptions{})
			if err != nil {
				return fmt.Errorf("Container stop %q:%w", container.ID, err)
			}
			_, err = d.client.ContainerRemove(ctx, container.ID, client.ContainerRemoveOptions{})
			if err != nil {
				return fmt.Errorf("Container remove %q:%w", container.ID, err)
			}
			fmt.Println("Container", container.Names, "removed")
		}
	}

	return nil

}
