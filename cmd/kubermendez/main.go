package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/alexflint/go-arg"
	"kuberMendez/deployment-parser"
	"kuberMendez/docker"
)

type ApplyCMD struct {
	File string `arg:"-f, required" help:"Apply a given deployment document"`
}

type ValidateCMD struct {
	File string `arg:"-f, required" help:"Validate a given deployment document, will not take any effect on the deployment itself"` //cambiar a deployment en vez de file
}

type GetCMD struct {
	Pods  *PodsCMD `arg:"subcommand: pods"`
}

type PodsCMD struct {
	DeploymentName string `arg:"-d" help:"List a specific deployment containers"`
	All bool			  `arg:"-A" help:"List all pods"`
}

type RemoveCMD struct {
	Deployment string `arg:"-f, required" help:"Deletes a given deployment containers"`
  
}

type args struct{
	Apply *ApplyCMD			`arg:"subcommand:apply, positional" help:"Used to create deployments"`
	Validate *ValidateCMD	`arg:"subcommand:validate" help:"Used to validate deployments before applying them"`
	Get *GetCMD				`arg:"subcommand:get"`
	Remove *RemoveCMD		`arg:"subcommand:remove"`
}

func (args) Version() string{
	return "Kubermendez v1.0" 
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func getFile(fileName string) []byte {
	absPath, err := filepath.Abs(fileName)
	check(err)

	data, err := os.ReadFile(absPath)
	check(err)

	return data
}

func main(){

	var args args
	arg.MustParse(&args)

	switch{
	case args.Apply != nil:
		file := getFile(args.Apply.File)
		parsed_yaml, err := parser.Parser(file)

		check(err)
		
		var deploymentName string = parsed_yaml.Metadata.Name
		var containers []parser.Container = parsed_yaml.Spec.Template.Spec.Containers

		for _, container := range containers{
			docker.DockerRun(container, deploymentName)
		}
		

		fmt.Println(parsed_yaml)

	case args.Validate != nil:
		if args.Validate.File != ""{
			file := getFile(args.Validate.File)
			status := parser.Validation(file)

			if status == nil{
				fmt.Println("OK")
			}else{
				fmt.Println("ERROR")
			}

		}
	case args.Get != nil:
		if args.Get.Pods.DeploymentName != ""{
			docker.ListContainers(args.Get.Pods.DeploymentName)
		} else if args.Get.Pods.All{
			docker.ListContainers("all")
		}
	case args.Remove != nil:
		docker.RemoveContainers(args.Remove.Deployment)
	}

}