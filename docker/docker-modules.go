package docker

import (
	"context"
	"fmt"
	"io"
	parser "kuberMendez/deployment-parser"
	"net/netip"
	"os"
	"strconv"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func Docker(spec parser.Container) {

	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	var image string = spec.Image
	exposedPorts := make(network.PortSet)
	hostPorts := make(network.PortMap)

	for _, port := range spec.Ports {
		p, err := network.ParsePort(fmt.Sprintf("%d/tcp", port.ContainerPort))
		if err != nil {
			panic(err)
		}
		exposedPorts[p] = struct{}{}
		hostIP, err := netip.ParseAddr("127.0.0.1")
		if err != nil{
			panic(err)
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
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Image: image,
		Name: spec.Name,
		Config: &container.Config{
			Labels: map[string]string{"creator":"Kubermendez"},
			ExposedPorts: exposedPorts,
		},
		HostConfig: &container.HostConfig{
			PortBindings: hostPorts,
		},
	})
	if err != nil {
		panic(err)
	}

	if startResult, err := apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		panic(err)
	}else{
		fmt.Println(startResult)
	}


	out, err := apiClient.ContainerLogs(ctx, resp.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}