package runtime

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	parser "kuberMendez/deployment-parser"
)

// ContainerState is the runtime-neutral view of a container observed by the
// control plane. Adapters are responsible for translating their native types
// and metadata into this structure.
type ContainerState struct {
	ID       string
	SpecName string
	SpecHash string
	Name     string
	Image    string
	Status   string
	Ports    []PortState
	Env      []EnvVar
}

//Needed for docker decoupling
type PortState struct {
	ContainerPort int  `json:"containerPort"`
	HostPort      bool `json:"hostPort"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ContainerSpecHash returns a stable hash for the desired fields that affect a
// running container. Slice order does not affect the result.
func ContainerSpecHash(spec parser.Container) string {
	normalized := spec

	normalized.Ports = append([]parser.Port(nil), spec.Ports...)
	if normalized.Ports == nil {
		normalized.Ports = []parser.Port{}
	}
	sort.Slice(normalized.Ports, func(i, j int) bool {
		if normalized.Ports[i].ContainerPort == normalized.Ports[j].ContainerPort {
			return !normalized.Ports[i].HostPort && normalized.Ports[j].HostPort
		}
		return normalized.Ports[i].ContainerPort < normalized.Ports[j].ContainerPort
	})

	normalized.Env = append([]parser.EnvVar(nil), spec.Env...)
	if normalized.Env == nil {
		normalized.Env = []parser.EnvVar{}
	}
	sort.Slice(normalized.Env, func(i, j int) bool {
		if normalized.Env[i].Name == normalized.Env[j].Name {
			return normalized.Env[i].Value < normalized.Env[j].Value
		}
		return normalized.Env[i].Name < normalized.Env[j].Name
	})

	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
