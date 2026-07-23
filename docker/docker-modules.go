package docker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"kuberMendez/deployment-parser"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"

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

type ContainerSummary struct {
	ID     string
	Labels map[string]string
	Name   string
	Image  string
	Status string
	Ports  []container.PortSummary
	Env    []parser.EnvVar
}

func initDockerClient() (client.APIClient, error) {
	apiClient, err := client.New(client.FromEnv)

	return apiClient, err
}

func DockerRun(ctx context.Context, spec parser.Container, deploymentName string, replicas int) error {
	ctx, close := context.WithTimeout(ctx, 60*time.Second)
	defer close()

	if replicas < 0 {
		return fmt.Errorf("replicas cannot be negative: %d", replicas)
	}

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

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

	reader, err := apiClient.ImagePull(ctx, fmt.Sprintf("docker.io/library/%v", image), client.ImagePullOptions{})
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			return fmt.Errorf("docker daemon not running: %w", err)

		} else if errdefs.IsNotFound(err) {
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
		resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
			Image: image,
			Name:  fmt.Sprintf("%v_%v", spec.Name, idGen.Generate()),
			Config: &container.Config{
				Labels: map[string]string{
					LabelCreator:           "Kubermendez",
					LabelDeploymentName:    deploymentName,
					LabelContainerName:     spec.Name,
					LabelContainerSpecHash: ContainerSpecHash(spec),
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

		if startResult, err := apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("start container %q: %w", spec.Name, err)
		} else {
			fmt.Println(startResult)
		}

		out, err := apiClient.ContainerLogs(ctx, resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return fmt.Errorf("Container lgos %q:%w", resp.ID, err)
		}

		stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	}

	return nil
}

func ListContainers(ctx context.Context, deploymentName string) ([]ContainerSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	apiClient, err := initDockerClient()
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	defer apiClient.Close()

	filters := make(client.Filters)

	if deploymentName == "all" {
		filters.Add("label", fmt.Sprintf("%s=Kubermendez", LabelCreator))
	} else {
		filters.Add(
			"label",
			fmt.Sprintf("%s=%s", LabelDeploymentName, deploymentName),
		)
	}

	containers, err := apiClient.ContainerList(
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
		return []ContainerSummary{}, nil
	}

	result := make([]ContainerSummary, 0, len(containers.Items))

	for _, container := range containers.Items {
		config, err := apiClient.ContainerInspect(ctx, container.ID, client.ContainerInspectOptions{})
		if err != nil {
			return nil, fmt.Errorf("inspect container %q: %w", container.ID, err)
		}

		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}

		result = append(result, ContainerSummary{
			ID:     container.ID,
			Labels: container.Labels,
			Name:   name,
			Image:  container.Image,
			Status: container.Status,
			Ports:  container.Ports,
			Env:    parseEnvVars(config.Container.Config.Env),
		})
	}

	return result, nil
}

func ContainerSpecHash(spec parser.Container) string {
	normalized := spec

	normalized.Ports = append([]parser.Port(nil), spec.Ports...)
	if normalized.Ports == nil {
		normalized.Ports = []parser.Port{}
	}
	sort.Slice(normalized.Ports, func(i, j int) bool {
		if normalized.Ports[i].ContainerPort == normalized.Ports[j].ContainerPort {
			return !normalized.Ports[i].HostPort && normalized.Ports[j].HostPort
		}
		return normalized.Ports[i].ContainerPort < normalized.Ports[j].ContainerPort
	})

	normalized.Env = append([]parser.EnvVar(nil), spec.Env...)
	if normalized.Env == nil {
		normalized.Env = []parser.EnvVar{}
	}
	sort.Slice(normalized.Env, func(i, j int) bool {
		if normalized.Env[i].Name == normalized.Env[j].Name {
			return normalized.Env[i].Value < normalized.Env[j].Value
		}
		return normalized.Env[i].Name < normalized.Env[j].Name
	})

	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func parseEnvVars(env []string) []parser.EnvVar {
	result := make([]parser.EnvVar, 0, len(env))
	for _, item := range env {
		name, value, _ := strings.Cut(item, "=")
		result = append(result, parser.EnvVar{
			Name:  name,
			Value: value,
		})
	}
	return result
}

func RemoveContainersByID(ctx context.Context, containerIDs []string) error {
	if len(containerIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer apiClient.Close()

	for _, containerID := range containerIDs {
		_, err := apiClient.ContainerStop(ctx, containerID, client.ContainerStopOptions{})
		if err != nil {
			return fmt.Errorf("container stop %q: %w", containerID, err)
		}
		_, err = apiClient.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{})
		if err != nil {
			return fmt.Errorf("container remove %q: %w", containerID, err)
		}
		fmt.Println("Container", containerID, "removed")
	}

	return nil
}

func RemoveContainers(ctx context.Context, deploymentName string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

	filters := make(client.Filters)
	filters.Add("label", fmt.Sprintf("%s=%v", LabelDeploymentName, deploymentName))

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{Filters: filters, All: true})
	if err != nil {
		return fmt.Errorf("Container list %w", err)
	}

	if len(containers.Items) == 0 {
		fmt.Println("No containers to delete")

	} else {

		for _, container := range containers.Items {
			_, err := apiClient.ContainerStop(ctx, container.ID, client.ContainerStopOptions{})
			if err != nil {
				return fmt.Errorf("Container stop %q:%w", container.ID, err)
			}
			_, err = apiClient.ContainerRemove(ctx, container.ID, client.ContainerRemoveOptions{})
			if err != nil {
				return fmt.Errorf("Container remove %q:%w", container.ID, err)
			}
			fmt.Println("Container", container.Names, "removed")
		}
	}

	return nil

}
