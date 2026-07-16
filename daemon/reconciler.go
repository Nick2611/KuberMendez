package daemon

import (
	"context"
	"fmt"
	apiserver "kuberMendez/api-server"
	"os"
	"time"

	"kuberMendez/deployment-parser"
	"kuberMendez/docker"
)

func InitReconcile(ctx context.Context, eventStream <-chan apiserver.ApplyRequestDto) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] Processing background task...\n", time.Now().Format("15:04:05"))
		case msg, ok := <-eventStream:
			if !ok {
				fmt.Println("Reconcile event stream closed")
				return
			}

			fmt.Println("working with", msg.Message)

			response := checkAppliedDeployment(ctx, msg)
			msg.Reply <- response

		case <-ctx.Done():
			fmt.Println("Worker received shutdown signal")
			return
		}
	}
}

//runs after the user applies a deployment file, this will trigger a channel notification that will
//start a process of parsing the deployment file and running its containers spec

func checkAppliedDeployment(ctx context.Context, req apiserver.ApplyRequestDto) apiserver.ReconcileResultDto {
	var fileName string = req.Message.DeploymentName
	var response apiserver.ReconcileResultDto

	data, err := os.ReadFile(fmt.Sprintf(".kubermendez/deployments/%v", fileName))
	if err != nil {
		response.DeploymentName = fileName
		response.Created = false
		response.Err = err

		return response
	}
	parsed_yaml, err := parser.Parser(data)
	if err != nil {
		fmt.Println(fmt.Errorf("Error parsing file %q: %w", fileName, err))
		response.DeploymentName = fileName
		response.Created = false
		response.Err = err

		return response
	}

	var deploymentName string = parsed_yaml.Metadata.Name
	var containers []parser.Container = parsed_yaml.Spec.Template.Spec.Containers
	var replicas int = parsed_yaml.Spec.Replicas

	response.DeploymentName = deploymentName

	for _, container := range containers {
		if err := docker.DockerRun(ctx, container, deploymentName, replicas); err != nil {
			response.Created = false
			response.Err = err
			return response
		}
		time.Sleep(10 * time.Second)
	}

	response.Created = true
	response.Err = nil
	return response
}

// Periodically runs after a set ammount of time, checks if a given container matches
func checkDesiredState(ctx context.Context) {

}
