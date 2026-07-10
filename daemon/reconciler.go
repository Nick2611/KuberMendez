package daemon

import(
	"context"
	"fmt"
	"time"

	"kuberMendez/deployment-parser"
	"kuberMendez/docker"
)

func InitReconcile(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] Processing background task...\n", time.Now().Format("15:04:05"))
		case <-ctx.Done():
			fmt.Println("Worker received shutdown signal. Cleaning up...")
			return
		}
	}
}

//runs after the user applies a deployment file, this will trigger a channel notification that will
//start a process of parsing the deployment file and running its containers spec

func checkAppliedDeployment(ctx context.Context, file []byte) { 
	parsed_yaml, err := parser.Parser(file)
	if err != nil{
		fmt.Println(fmt.Errorf("Error parsing file %q: %w", file, err))
	}
	
	var deploymentName string = parsed_yaml.Metadata.Name
	var containers []parser.Container = parsed_yaml.Spec.Template.Spec.Containers

	for _, container := range containers{
		docker.DockerRun(ctx, container, deploymentName)
	}
}


//Periodically runs after a set ammount of time, checks if a given container matches
func checkDesiredState(ctx context.Context){

}