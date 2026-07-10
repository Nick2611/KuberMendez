package docker

import (
	"context"
	"fmt"
	"io"
	"kuberMendez/deployment-parser"
	"log"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/docker/docker/errdefs"
)
func catchDockerNotRunningError(){
	log.Fatal("Docker Daemon not running.") //TODO Usar otra cosa que no sea log.Fatal
}

func initDockerClient() (client.APIClient, error) {
	apiClient, err := client.New(client.FromEnv)

	return apiClient, err
}

func DockerRun(ctx context.Context, spec parser.Container, deploymentName string) error {
	ctx, close := context.WithTimeout(ctx, 60 * time.Second)
	defer close()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

	var image string = spec.Image
	var envList []string

	if len(spec.Env) != 0{
		for _, env := range spec.Env{
			envList = append(envList, fmt.Sprintf("%v=%v", env.Name, env.Value))
		}
	}

	exposedPorts := make(network.PortSet)
	hostPorts := make(network.PortMap)

	for _, port := range spec.Ports {
		p, err := network.ParsePort(fmt.Sprintf("%d/tcp", port.ContainerPort))
		if err != nil {
			return fmt.Errorf("Parse port %d:%q", port.ContainerPort, err)
		}
		exposedPorts[p] = struct{}{}
		hostIP, err := netip.ParseAddr("127.0.0.1")
		if err != nil{
			return fmt.Errorf("Parse address %q:%w", hostIP, err)
		}

		if port.HostPort != nil{
			hostPort := network.PortBinding{
				HostIP: hostIP,
				HostPort: strconv.Itoa(*port.HostPort),
			}
			hostPorts[p] = append(hostPorts[p], hostPort)

		}

	}

	reader, err := apiClient.ImagePull(ctx, fmt.Sprintf("docker.io/library/%v", image), client.ImagePullOptions{})
	if err != nil {
		if client.IsErrConnectionFailed(err){
			catchDockerNotRunningError()
			return err

		} else if errdefs.NotFound(err) != nil {
			fmt.Println("Image not found", image)
			return fmt.Errorf("pull image %q: %w", image, err)
		}
		return err
	}
	io.Copy(os.Stdout, reader)

	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Image: image,
		Name: spec.Name,
		Config: &container.Config{
			Labels: map[string]string{
				"creator": "Kubermendez",
				"DeploymentName": deploymentName,
			},
			ExposedPorts: exposedPorts,
			Env: envList,
		},
		HostConfig: &container.HostConfig{
			PortBindings: hostPorts,
		},
	})
	if err != nil {
		return fmt.Errorf("create container %q: %w", spec.Name, err)
	}

	if startResult, err := apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("start container %q: %w", spec.Name, err)
	}else{
		fmt.Println(startResult)
	}


	out, err := apiClient.ContainerLogs(ctx, resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return fmt.Errorf("Container lgos %q:%w", resp.ID, err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	return nil
}

func ListContainers(deploymentName string) error {

	ctx := context.Background()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

	filters := make(client.Filters)

	if deploymentName != "all"{
		filters.Add("label",fmt.Sprintf("DeploymentName=%v",deploymentName))

	} else{
		filters.Add("label","creator=Kubermendez")
	}

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{Filters: filters, All: true})
	if err != nil {
		return fmt.Errorf("Container list %w", err)
	}


	if len(containers.Items) == 0 && deploymentName != "all"{
		fmt.Println("There are no containers associated with that deployment name")
	} else if len(containers.Items) == 0 && deploymentName == "all"{
		fmt.Println("There are no containers up")
	}

	for _, container := range containers.Items {
		fmt.Println(container.Names) 
	}

	return nil

}

func RemoveContainers(deploymentName string) error {
	ctx := context.Background()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

	filters := make(client.Filters)
	filters.Add("label",fmt.Sprintf("DeploymentName=%v",deploymentName))

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{Filters: filters, All: true})
	if err != nil {
		return fmt.Errorf("Container list %w", err)
	}

	if len(containers.Items) == 0{
		fmt.Println("No containers to delete")

	} else {

		for _, container := range containers.Items {
			_, err := apiClient.ContainerStop(ctx, container.ID, client.ContainerStopOptions{})
			if err != nil{
				return fmt.Errorf("Container stop %q:%w", container.ID, err)
			}
			_, err = apiClient.ContainerRemove(ctx, container.ID, client.ContainerRemoveOptions{})
			if err != nil{
				return fmt.Errorf("Container remove %q:%w", container.ID, err)
			}
			fmt.Println("Container", container.Names, "removed")
		}
	}

	return nil

}