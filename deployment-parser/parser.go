package parser

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"regexp"
)

type Manifest struct {
	APIVersion string         `yaml:"apiVersion" validate:"required"`
	Kind       string         `yaml:"kind" validate:"required"`
	Metadata   Metadata       `yaml:"metadata" validate:"required"`
	Spec       DeploymentSpec `yaml:"spec" validate:"required"`
}

type Metadata struct {
	Name   string	`yaml:"name" validate:"required"`
	Labels struct{
		App string `yaml:"app"`
	}	`yaml:"labels"`
}

type DeploymentSpec struct {
	Replicas int      `yaml:"replicas"` //should validate this more rigorously 
	Selector Selector `yaml:"selector" validate:"required"`
	Template Template `yaml:"template" validate:"required"` 
}

type Selector struct {
	MatchLabels map[string]string `yaml:"matchLabels" validate:"required"`
}

type Template struct {
	Metadata TemplateMetadata `yaml:"metadata" validate:"required"`
	Spec     TemplateSpec     `yaml:"spec" validate:"required"`
}

type TemplateMetadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels" validate:"required"` //TODO quizas reemplazar por struct matchLabels
}

type TemplateSpec struct {
	Containers []Container `yaml:"containers" validate:"required,dive"`
}

type Container struct {
	Name  string `yaml:"name" validate:"required"`
	Image string `yaml:"image" validate:"required"`
	Ports []Port `yaml:"ports" validate:"dive"`
	Env   []EnvVar `yaml:"env" validate:"dive"`
}

type Port struct {
	ContainerPort int `yaml:"containerPort"`
	HostPort bool	  `yaml:"hostPort"`
}

type EnvVar struct {
	Name  string `yaml:"name" validate:"required"`
	Value string `yaml:"value" validate:"required"`
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func getParsedManifest(file []byte) Manifest {
	var manifest Manifest

	err := yaml.Unmarshal(file, &manifest)
	if err != nil{
		return Manifest{}
	}

	return manifest
	
}

func Parser(file []byte) (Manifest, error){
	manifest := getParsedManifest(file)
	if err:= Validation(file); err != nil{
		return Manifest{}, err
	}

	fmt.Printf("%+v\n", manifest)

	return manifest, nil
}

func Validation(file []byte) error{
	validate := validator.New(validator.WithRequiredStructEnabled())

	manifest := getParsedManifest(file)

	if err := validate.Struct(manifest); err != nil {
		return err
	}

	var dnsLabelRegexp = regexp.MustCompile(`^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$`)

	for _, pod := range manifest.Spec.Template.Spec.Containers {
		if !dnsLabelRegexp.MatchString(pod.Name) {
			return fmt.Errorf("invalid container name: %q", pod.Name)
		}
	}

	return nil

}