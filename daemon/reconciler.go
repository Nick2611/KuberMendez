package daemon

import (
	"context"
	"fmt"
	"kuberMendez/events"
	"os"
	"time"

	"kuberMendez/deployment-parser"
	"kuberMendez/docker"
)

func InitReconcile(ctx context.Context, eventStream <-chan events.Message) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] Processing background task...\n", time.Now().Format("15:04:05"))
		case msg := <-eventStream:
			fmt.Println("working with", msg.DeploymentName)
			checkAppliedDeployment(ctx, msg.DeploymentName)
		case <-ctx.Done():
			fmt.Println("Worker received shutdown signal. Cleaning up...")
			return
		}
	}
}

//runs after the user applies a deployment file, this will trigger a channel notification that will
//start a process of parsing the deployment file and running its containers spec

func checkAppliedDeployment(ctx context.Context, fileName string) {
	data, err := os.ReadFile(fmt.Sprintf(".kubermendez/deployments/%v", fileName))
	parsed_yaml, err := parser.Parser(data)
	if err != nil {
		fmt.Println(fmt.Errorf("Error parsing file %q: %w", fileName, err))
	}

	var deploymentName string = parsed_yaml.Metadata.Name
	var containers []parser.Container = parsed_yaml.Spec.Template.Spec.Containers
	var replicas int = parsed_yaml.Spec.Replicas

	for _, container := range containers {
		docker.DockerRun(ctx, container, deploymentName, replicas)
		time.Sleep(10 * time.Second)
	}
}

// Periodically runs after a set ammount of time, checks if a given container matches
func checkDesiredState(ctx context.Context) {

}
