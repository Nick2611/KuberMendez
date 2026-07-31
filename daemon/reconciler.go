package daemon

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	apiserver "kuberMendez/api-server"
	"kuberMendez/deployment-parser"
	runtimecontract "kuberMendez/runtime"
	"kuberMendez/utils"
)

type ReconcileRuntime interface {
	ListContainers(ctx context.Context, deploymentName string) ([]runtimecontract.ContainerState, error)
	RemoveContainersByID(ctx context.Context, containerIDs []string) error
	ContainerRun(ctx context.Context, spec parser.Container, deploymentName string, replicas int) error
}

func InitReconcile(ctx context.Context, eventStream <-chan apiserver.ApplyRequestDto, rr ReconcileRuntime) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[%s] Processing background task...\n", time.Now().Format("15:04:05"))
			status, err := checkDesiredState(ctx, rr)

			switch {
			case status == true:
				fmt.Println("Deployment drift detected, reconcile executed")

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

			response := checkAppliedDeployment(ctx, msg, rr)
			msg.Reply <- response

		case <-ctx.Done():
			fmt.Println("Worker received shutdown signal")
			return
		}
	}
}

//runs after the user applies a deployment file, this will trigger a channel notification that will
//start a process of parsing the deployment file and running its containers spec

func checkAppliedDeployment(ctx context.Context, req apiserver.ApplyRequestDto, rr ReconcileRuntime) apiserver.ReconcileResultDto {
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
	changed, err := workCurrentDeployment(reconcileCtx, deploymentName, containers, parsed_yaml.Spec.Replicas, rr)
	cancel()
	if err != nil {
		response.DeploymentName = deploymentName
		response.Created = false
		response.Err = err
		return response
	}

	response.DeploymentName = deploymentName
	response.Created = changed
	response.Err = nil

	return response
}

// Periodically runs after a set ammount of time, checks if a given deployment matches container desired state
// Sequentially at first, might add concurrency later on
func checkDesiredState(ctx context.Context, rr ReconcileRuntime) (bool, error) {

	files, err := utils.GetFiles(utils.DefaultDeploymentsDirectory)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		parsed_yaml, err := parser.Parser(file)
		if err != nil {
			fmt.Println(fmt.Errorf("Error parsing file %q: %w", file, err))
			return false, err
		}

		reconcileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		changed, err := workCurrentDeployment(
			reconcileCtx,
			parsed_yaml.Metadata.Name,
			parsed_yaml.Spec.Template.Spec.Containers,
			parsed_yaml.Spec.Replicas,
			rr,
		)
		cancel()
		if err != nil {
			return false, err
		}
		if changed {
			return true, nil
		}

	}

	return false, nil

}

// Deletes and recreates containers that differ from the stored desired state.
func workCurrentDeployment(ctx context.Context, deploymentName string, desired []parser.Container, replicas int, rr ReconcileRuntime) (bool, error) {
	if deploymentName == "" {
		return false, fmt.Errorf("deployment name is required")
	}
	if len(desired) == 0 {
		return false, fmt.Errorf("no container specs available")
	}
	if replicas < 0 {
		return false, fmt.Errorf("replicas cannot be negative: %d", replicas)
	}

	actual, err := rr.ListContainers(ctx, deploymentName)
	if err != nil {
		return false, err
	}

	if replicas == 0 {
		if len(actual) == 0 {
			return false, nil
		}
		return true, rr.RemoveContainersByID(ctx, containerIDs(actual))
	}

	desiredByName := make(map[string]parser.Container, len(desired))
	for _, spec := range desired {
		if _, ok := desiredByName[spec.Name]; ok {
			return false, fmt.Errorf("duplicate container name %q", spec.Name)
		}
		desiredByName[spec.Name] = spec
	}

	actualByName := make(map[string][]runtimecontract.ContainerState, len(desiredByName))
	for _, actualContainer := range actual {
		specName := actualContainerSpecName(actualContainer)
		actualByName[specName] = append(actualByName[specName], actualContainer)
	}

	changed := false
	for specName, runningContainers := range actualByName {
		if _, ok := desiredByName[specName]; ok {
			continue
		}
		if err := rr.RemoveContainersByID(ctx, containerIDs(runningContainers)); err != nil {
			return changed, err
		}
		delete(actualByName, specName)
		changed = true
	}

	for specName, spec := range desiredByName {
		runningContainers := actualByName[specName]
		sort.Slice(runningContainers, func(i, j int) bool {
			return runningContainers[i].ID < runningContainers[j].ID
		})

		switch {
		case len(runningContainers) == 0:
			if err := rr.ContainerRun(ctx, spec, deploymentName, replicas); err != nil {
				return changed, err
			}
			changed = true
		case !allContainersMatchDesired(runningContainers, spec):
			if err := rr.RemoveContainersByID(ctx, containerIDs(runningContainers)); err != nil {
				return changed, err
			}
			if err := rr.ContainerRun(ctx, spec, deploymentName, replicas); err != nil {
				return changed, err
			}
			changed = true
		case len(runningContainers) < replicas:
			if err := rr.ContainerRun(ctx, spec, deploymentName, replicas-len(runningContainers)); err != nil {
				return changed, err
			}
			changed = true
		case len(runningContainers) > replicas:
			if err := rr.RemoveContainersByID(ctx, containerIDs(runningContainers[replicas:])); err != nil {
				return changed, err
			}
			changed = true
		}
	}

	return changed, nil
}

func actualContainerSpecName(actual runtimecontract.ContainerState) string {
	if actual.SpecName != "" {
		return actual.SpecName
	}

	name := strings.TrimPrefix(actual.Name, "/")
	if idx := strings.LastIndex(name, "_"); idx > 0 {
		return name[:idx]
	}
	return name
}

func allContainersMatchDesired(actual []runtimecontract.ContainerState, desired parser.Container) bool {
	for _, actualContainer := range actual {
		if !containerMatchesDesired(actualContainer, desired) {
			return false
		}
	}
	return true
}

func containerMatchesDesired(actual runtimecontract.ContainerState, desired parser.Container) bool {
	if actual.SpecHash != "" {
		return actual.SpecHash == runtimecontract.ContainerSpecHash(desired)
	}

	return actual.Image == desired.Image &&
		portsMatch(desired.Ports, actual.Ports) &&
		envVarsMatch(desired.Env, actual.Env)
}

func portsMatch(desired []parser.Port, actual []runtimecontract.PortState) bool {
	if len(desired) != len(actual) {
		return false
	}

	desiredPorts := make(map[int]bool, len(desired))
	for _, port := range desired {
		desiredPorts[port.ContainerPort] = port.HostPort
	}

	actualPorts := make(map[int]bool, len(actual))
	for _, port := range actual {
		actualPorts[port.ContainerPort] = port.HostPort
	}

	if len(desiredPorts) != len(actualPorts) {
		return false
	}

	for containerPort, wantsHostPort := range desiredPorts {
		hasHostPort, ok := actualPorts[containerPort]
		if !ok || hasHostPort != wantsHostPort {
			return false
		}
	}
	return true
}

func envVarsMatch(desired []parser.EnvVar, actual []runtimecontract.EnvVar) bool {
	if len(desired) != len(actual) {
		return false
	}

	actualEnv := make(map[string]string, len(actual))
	for _, env := range actual {
		actualEnv[env.Name] = env.Value
	}

	if len(desired) != len(actualEnv) {
		return false
	}

	for _, env := range desired {
		value, ok := actualEnv[env.Name]
		if !ok || value != env.Value {
			return false
		}
	}
	return true
}

func containerIDs(containers []runtimecontract.ContainerState) []string {
	ids := make([]string, 0, len(containers))
	for _, container := range containers {
		ids = append(ids, container.ID)
	}
	return ids
}
