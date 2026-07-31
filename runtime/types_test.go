package runtime

import (
	"testing"

	parser "kuberMendez/deployment-parser"
)

func TestContainerSpecHashIsStableAcrossSliceOrder(t *testing.T) {
	first := parser.Container{
		Name:  "web",
		Image: "nginx",
		Ports: []parser.Port{
			{ContainerPort: 443, HostPort: true},
			{ContainerPort: 80},
		},
		Env: []parser.EnvVar{
			{Name: "MODE", Value: "production"},
			{Name: "LOG_LEVEL", Value: "info"},
		},
	}
	second := parser.Container{
		Name:  "web",
		Image: "nginx",
		Ports: []parser.Port{
			{ContainerPort: 80},
			{ContainerPort: 443, HostPort: true},
		},
		Env: []parser.EnvVar{
			{Name: "LOG_LEVEL", Value: "info"},
			{Name: "MODE", Value: "production"},
		},
	}

	if got, want := ContainerSpecHash(first), ContainerSpecHash(second); got != want {
		t.Fatalf("hashes differ for equivalent specs: %q != %q", got, want)
	}
}

func TestContainerSpecHashChangesWithDesiredState(t *testing.T) {
	base := parser.Container{Name: "web", Image: "nginx:1"}
	changed := parser.Container{Name: "web", Image: "nginx:2"}

	if got, wantDifferent := ContainerSpecHash(base), ContainerSpecHash(changed); got == wantDifferent {
		t.Fatalf("hash did not change when image changed: %q", got)
	}
}
