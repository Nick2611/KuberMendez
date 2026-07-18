package daemon

import (
	"context"
	"fmt"
	apiserver "kuberMendez/api-server"
	"os"
	"time"

	"kuberMendez/deployment-parser"
	"kuberMendez/docker"
	"kuberMendez/utils"
)

func InitReconcile(ctx context.Context, eventStream <-chan apiserver.ApplyRequestDto) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] Processing background task...\n", time.Now().Format("15:04:05"))
			status, err := checkDesiredState(ctx)

			switch{
			case status == true:
				fmt.Println("Replicas difference detected, reconcile executed")

			case status == false && err == nil:
				fmt.Println("Deployment healthy, not reconciled")

			case status == false && err != nil:
				fmt.Println("error:", err)
			}

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

	reconcileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	actual, err := docker.ListContainers(reconcileCtx, deploymentName)
	if err != nil {
		return apiserver.ReconcileResultDto{
			DeploymentName: deploymentName,
			Created: false,
			Err: err,
		}
	}

	var replicas int = parsed_yaml.Spec.Replicas - len(actual)


	for _, container := range containers {
		if err := docker.DockerRun(ctx, container, deploymentName, replicas); err != nil {
			response.Created = false
			response.Err = err
			return response
		}
		time.Sleep(10 * time.Second)
	}

	response.DeploymentName = deploymentName
	response.Created = true
	response.Err = nil

	return response
}

// Periodically runs after a set ammount of time, checks if a given deployment matches container desired state
// Sequentially at first, might add concurrency later on
func checkDesiredState(ctx context.Context) (bool, error) {

	files, err := utils.GetFiles(utils.DefaultDeploymentsDirectory)
	if err != nil{
		return false, err
	}

	for _, file := range files{
		parsed_yaml, err := parser.Parser(file)
		if err != nil {
			fmt.Println(fmt.Errorf("Error parsing file %q: %w", file, err))
			return false, err
		}
		var deploymentName string = parsed_yaml.Metadata.Name //TODO Encapsulate into function (same behavior as checkapplied)
		var containers []parser.Container = parsed_yaml.Spec.Template.Spec.Containers

		reconcileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		actual, err := docker.ListContainers(reconcileCtx, deploymentName)
		if err != nil {
			return false, err
		}

		var replicas int = parsed_yaml.Spec.Replicas - len(actual)

		if replicas != 0{

			for _, container := range containers {
				if err := docker.DockerRun(ctx, container, deploymentName, replicas); err != nil {
					return false, err
				}
				time.Sleep(10 * time.Second)
			}

			return true, nil
		}

	}

	return false, nil

}
