package daemon

import (
	"context"
	"errors"
	"reflect"
	"testing"

	parser "kuberMendez/deployment-parser"
	runtimecontract "kuberMendez/runtime"
)

type runCall struct {
	spec           parser.Container
	deploymentName string
	replicas       int
}

type fakeReconcileRuntime struct {
	containers []runtimecontract.ContainerState
	listErr    error
	removeErr  error
	runErr     error
	removed    [][]string
	runs       []runCall
}

func (f *fakeReconcileRuntime) ListContainers(_ context.Context, _ string) ([]runtimecontract.ContainerState, error) {
	return append([]runtimecontract.ContainerState(nil), f.containers...), f.listErr
}

func (f *fakeReconcileRuntime) RemoveContainersByID(_ context.Context, ids []string) error {
	f.removed = append(f.removed, append([]string(nil), ids...))
	return f.removeErr
}

func (f *fakeReconcileRuntime) ContainerRun(
	_ context.Context,
	spec parser.Container,
	deploymentName string,
	replicas int,
) error {
	f.runs = append(f.runs, runCall{
		spec:           spec,
		deploymentName: deploymentName,
		replicas:       replicas,
	})
	return f.runErr
}

func TestWorkCurrentDeploymentLeavesMatchingStateAlone(t *testing.T) {
	spec := parser.Container{
		Name:  "web",
		Image: "nginx",
		Ports: []parser.Port{{ContainerPort: 80, HostPort: true}},
		Env:   []parser.EnvVar{{Name: "MODE", Value: "production"}},
	}
	runtime := &fakeReconcileRuntime{
		containers: []runtimecontract.ContainerState{
			matchingContainer("one", spec),
			matchingContainer("two", spec),
		},
	}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{spec}, 2, runtime)
	if err != nil {
		t.Fatalf("reconcile matching state: %v", err)
	}
	if changed {
		t.Fatal("matching state reported as changed")
	}
	if len(runtime.runs) != 0 || len(runtime.removed) != 0 {
		t.Fatalf("matching state produced actions: runs=%v removed=%v", runtime.runs, runtime.removed)
	}
}

func TestWorkCurrentDeploymentScalesUp(t *testing.T) {
	spec := parser.Container{Name: "web", Image: "nginx"}
	runtime := &fakeReconcileRuntime{
		containers: []runtimecontract.ContainerState{matchingContainer("one", spec)},
	}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{spec}, 3, runtime)
	if err != nil {
		t.Fatalf("scale up: %v", err)
	}
	if !changed {
		t.Fatal("scale up was not reported as changed")
	}
	if len(runtime.runs) != 1 || runtime.runs[0].replicas != 2 {
		t.Fatalf("run calls = %+v, want one call for two replicas", runtime.runs)
	}
}

func TestWorkCurrentDeploymentScalesDownDeterministically(t *testing.T) {
	spec := parser.Container{Name: "web", Image: "nginx"}
	runtime := &fakeReconcileRuntime{
		containers: []runtimecontract.ContainerState{
			matchingContainer("three", spec),
			matchingContainer("one", spec),
			matchingContainer("two", spec),
		},
	}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{spec}, 1, runtime)
	if err != nil {
		t.Fatalf("scale down: %v", err)
	}
	if !changed {
		t.Fatal("scale down was not reported as changed")
	}
	want := [][]string{{"three", "two"}}
	if !reflect.DeepEqual(runtime.removed, want) {
		t.Fatalf("removed = %v, want %v", runtime.removed, want)
	}
}

func TestWorkCurrentDeploymentReplacesDriftedState(t *testing.T) {
	desired := parser.Container{Name: "web", Image: "nginx:2"}
	actual := runtimecontract.ContainerState{
		ID:       "old",
		SpecName: "web",
		SpecHash: runtimecontract.ContainerSpecHash(parser.Container{Name: "web", Image: "nginx:1"}),
		Image:    "nginx:1",
	}
	runtime := &fakeReconcileRuntime{containers: []runtimecontract.ContainerState{actual}}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{desired}, 1, runtime)
	if err != nil {
		t.Fatalf("replace drifted state: %v", err)
	}
	if !changed {
		t.Fatal("drift replacement was not reported as changed")
	}
	if !reflect.DeepEqual(runtime.removed, [][]string{{"old"}}) {
		t.Fatalf("removed = %v, want old container", runtime.removed)
	}
	if len(runtime.runs) != 1 || runtime.runs[0].spec.Image != "nginx:2" || runtime.runs[0].replicas != 1 {
		t.Fatalf("run calls = %+v, want replacement using desired spec", runtime.runs)
	}
}

func TestWorkCurrentDeploymentRemovesObsoleteSpec(t *testing.T) {
	desired := parser.Container{Name: "web", Image: "nginx"}
	obsolete := parser.Container{Name: "worker", Image: "busybox"}
	runtime := &fakeReconcileRuntime{
		containers: []runtimecontract.ContainerState{
			matchingContainer("web-one", desired),
			matchingContainer("worker-one", obsolete),
		},
	}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{desired}, 1, runtime)
	if err != nil {
		t.Fatalf("remove obsolete spec: %v", err)
	}
	if !changed {
		t.Fatal("obsolete spec removal was not reported as changed")
	}
	if !reflect.DeepEqual(runtime.removed, [][]string{{"worker-one"}}) {
		t.Fatalf("removed = %v, want obsolete worker container", runtime.removed)
	}
	if len(runtime.runs) != 0 {
		t.Fatalf("obsolete spec removal unexpectedly created containers: %+v", runtime.runs)
	}
}

func TestWorkCurrentDeploymentRemovesAllContainersAtZeroReplicas(t *testing.T) {
	spec := parser.Container{Name: "web", Image: "nginx"}
	runtime := &fakeReconcileRuntime{
		containers: []runtimecontract.ContainerState{
			matchingContainer("one", spec),
			matchingContainer("two", spec),
		},
	}

	changed, err := workCurrentDeployment(context.Background(), "demo", []parser.Container{spec}, 0, runtime)
	if err != nil {
		t.Fatalf("scale to zero: %v", err)
	}
	if !changed {
		t.Fatal("scale to zero was not reported as changed")
	}
	if !reflect.DeepEqual(runtime.removed, [][]string{{"one", "two"}}) {
		t.Fatalf("removed = %v, want all containers", runtime.removed)
	}
	if len(runtime.runs) != 0 {
		t.Fatalf("scale to zero unexpectedly created containers: %+v", runtime.runs)
	}
}

func TestWorkCurrentDeploymentPropagatesRuntimeError(t *testing.T) {
	expected := errors.New("runtime unavailable")
	runtime := &fakeReconcileRuntime{listErr: expected}

	changed, err := workCurrentDeployment(
		context.Background(),
		"demo",
		[]parser.Container{{Name: "web", Image: "nginx"}},
		1,
		runtime,
	)
	if changed {
		t.Fatal("failed reconciliation reported as changed")
	}
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v, want %v", err, expected)
	}
}

func matchingContainer(id string, spec parser.Container) runtimecontract.ContainerState {
	ports := make([]runtimecontract.PortState, 0, len(spec.Ports))
	for _, port := range spec.Ports {
		ports = append(ports, runtimecontract.PortState{
			ContainerPort: port.ContainerPort,
			HostPort:      port.HostPort,
		})
	}

	env := make([]runtimecontract.EnvVar, 0, len(spec.Env))
	for _, variable := range spec.Env {
		env = append(env, runtimecontract.EnvVar{
			Name:  variable.Name,
			Value: variable.Value,
		})
	}

	return runtimecontract.ContainerState{
		ID:       id,
		SpecName: spec.Name,
		SpecHash: runtimecontract.ContainerSpecHash(spec),
		Name:     spec.Name + "_" + id,
		Image:    spec.Image,
		Ports:    ports,
		Env:      env,
	}
}
