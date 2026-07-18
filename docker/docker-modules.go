package docker

import (
	"context"
	"fmt"
	"io"
	"kuberMendez/deployment-parser"
	"log"
	"net/netip"
	"os"
	"time"


	"github.com/docker/docker/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/devjefster/GoShortUniqueID/idgen"
)

type ContainerSummary struct {
	ID     string
	Labels map[string]string
	Names  []string
	Image  string
	Status string
	Ports  []container.PortSummary
}

func catchDockerNotRunningError(){
	log.Fatal("Docker Daemon not running.") //TODO Usar otra cosa que no sea log.Fatal
}

func initDockerClient() (client.APIClient, error) {
	apiClient, err := client.New(client.FromEnv)

	return apiClient, err
}

func DockerRun(ctx context.Context, spec parser.Container, deploymentName string, replicas int) error {
	ctx, close := context.WithTimeout(ctx, 60 * time.Second)
	defer close()

	apiClient, err := initDockerClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	defer apiClient.Close()

	var image string = spec.Image
	var envList []string

	idGen := idgen.New(6, "", "")

	if len(spec.Env) != 0{
		for _, env := range spec.Env{
			envList = append(envList, fmt.Sprintf("%v=%v", env.Name, env.Value))
		}
	}

	portBindings := make(network.PortMap)

	for _, port := range spec.Ports {
		p, err := network.ParsePort(fmt.Sprintf("%d/tcp", port.ContainerPort))
		if err != nil {
			return fmt.Errorf("Parse port %d:%q", port.ContainerPort, err)
		}
		portBindings[p] = []network.PortBinding{}
		hostIP, err := netip.ParseAddr("127.0.0.1")
		if err != nil{
			return fmt.Errorf("Parse address %q:%w", hostIP, err)
		}

		if port.HostPort{
			hostPort := network.PortBinding{
				HostIP: hostIP,
				HostPort: "",
			}
			portBindings[p] = append(portBindings[p], hostPort)

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

	for i := 1; i <= replicas; i++{
		resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
			Image: image,
			Name: fmt.Sprintf("%v_%v", spec.Name, idGen.Generate()),
			Config: &container.Config{
				Labels: map[string]string{
					"Creator": "Kubermendez",
					"DeploymentName": deploymentName,
				},
				Env: envList,
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
		}else{
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

func ListContainers(ctx context.Context, deploymentName string,) ([]ContainerSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	apiClient, err := initDockerClient()
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	defer apiClient.Close()

	filters := make(client.Filters)

	if deploymentName == "all" {
		filters.Add("label", "creator=Kubermendez")
	} else {
		filters.Add(
			"label",
			fmt.Sprintf("DeploymentName=%s", deploymentName),
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

	result := make([]ContainerSummary, 0, len(containers.Items))

	for _, container := range containers.Items {
		result = append(result, ContainerSummary{
			ID:     container.ID,
			Labels: container.Labels,
			Names:  container.Names,
			Image:  container.Image,
			Status: container.Status,
			Ports:  container.Ports,
		})
	}

	return result, nil
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