package main

import (
	"kuberMendez/daemon"
	"kuberMendez/deployment-parser"

	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	// "log"
	"strings"
	"strconv"

	"github.com/alexflint/go-arg"
)

type ApplyCMD struct {
	File string `arg:"-f, required" help:"Apply a given deployment document"`
}

type ValidateCMD struct {
	File string `arg:"-f, required" help:"Validate a given deployment document, will not take any effect on the deployment itself"` //cambiar a deployment en vez de file
}

type GetCMD struct {
	Pods *PodsCMD `arg:"subcommand: pods"`
}

type PodsCMD struct {
	DeploymentName string `arg:"-d" help:"List a specific deployment containers"`
	All            bool   `arg:"-A" help:"List all pods"`
}

type RemoveCMD struct {
	Deployment string `arg:"-f, required" help:"Deletes a given deployment containers"`
}

type StopCMD struct {}

type InitCMD struct{}

type DaemonCMD struct{}

type args struct { //TODO Aditional stop command, goroutines don't stop with ctrl c (separate process)
	Apply    *ApplyCMD    `arg:"subcommand:apply, positional" help:"Used to create deployments"`
	Validate *ValidateCMD `arg:"subcommand:validate" help:"Used to validate deployments before applying them"`
	Get      *GetCMD      `arg:"subcommand:get"`
	Remove   *RemoveCMD   `arg:"subcommand:remove"`
	Init     *InitCMD     `arg:"subcommand:init" help:"Boot KuberMendez daemon"`
	Daemon   *DaemonCMD   `arg:"subcommand:daemon" help:"Run the KuberMendez daemon process"`
	Stop 	 *StopCMD	  `arg:"subcommand:stop" help:"Stop the KuberMendez daemon"`
}

func (args) Version() string {
	return "Kubermendez v1.0"
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func getFile(fileName string) ([]byte, error) {
	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return nil, fmt.Errorf("Bad filepath %q: %w", fileName, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("Read file %q: %w", absPath, err)
	}

	return data, nil
}

const (
	deploymentsDirectory = ".kubermendez/deployments"
	daemonPidPath = ".kubermendez/daemon.pid"
)

func deploymentStatePath(deploymentName string) (string, error) {
	if deploymentName == "" {
		return "", fmt.Errorf("deployment metadata.name is required")
	}
	if filepath.Base(deploymentName) != deploymentName {
		return "", fmt.Errorf("deployment metadata.name %q cannot contain path separators", deploymentName)
	}

	return filepath.Join(deploymentsDirectory, deploymentName+".yaml"), nil
}

func fileToInt() (int,error){
	bytes, err := os.ReadFile(daemonPidPath)
	if err != nil {
		return -1, fmt.Errorf("failed to read file: %v", err)
	}

	content := strings.TrimSpace(string(bytes))

	// Parse string to int
	num, err := strconv.Atoi(content)
	if err != nil {
		return -1, fmt.Errorf("failed to parse int: %v", err)
	}

	return num, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var args args
	arg.MustParse(&args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case args.Init != nil:

		if _, err := os.Stat(daemonPidPath); err == nil {
			return fmt.Errorf("Error, daemon already running")
		} else {
			pid, err := daemon.StartBackground()
			if err != nil {
				return err
			}
			fmt.Printf("KuberMendez daemon started with PID %d\n", pid)
		}

	case args.Daemon != nil:
		fmt.Println("KuberMendez daemon running")
		daemon.InitDaemon(ctx)
		
	
	case args.Stop != nil:

		if _, err := os.Stat(daemonPidPath); err != nil {
			return fmt.Errorf("Error, daemon is not running")
		} else {
			num, err := fileToInt()
			if err != nil{
				panic(err)
			}
			process, err := os.FindProcess(num)
			if err != nil {
				fmt.Printf("Failed to find process: %v\n", err)
			}
			err = process.Signal(syscall.SIGTERM)
			if err != nil {
				fmt.Printf("Failed to send SIGTERM: %v\n", err)
			}

			fmt.Printf("Process %v finished\n", num)
		}

	case args.Apply != nil:
		file, err := getFile(args.Apply.File)
		if err != nil {
			return err
		}

		manifest, err := parser.Parser(file)
		if err != nil {
			return fmt.Errorf("parse deployment manifest: %w", err)
		}

		statePath, err := deploymentStatePath(manifest.Metadata.Name)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(deploymentsDirectory, 0755); err != nil {
			return fmt.Errorf("create deployments state directory: %w", err)
		}
		if err := os.WriteFile(statePath, file, 0644); err != nil {
			return fmt.Errorf("write deployment state: %w", err)
		}
		fmt.Printf("Deployment %q saved to %s\n", manifest.Metadata.Name, statePath)

		return nil

	case args.Validate != nil:
		if args.Validate.File != "" {
			file, err := getFile(args.Validate.File)
			if err != nil {
				return err
			}
			status := parser.Validation(file)

			if status == nil {
				fmt.Println("OK")
			} else {
				fmt.Println("ERROR")
			}

		}
		// case args.Get != nil:
		// 	if args.Get.Pods.DeploymentName != ""{
		// 		docker.ListContainers(args.Get.Pods.DeploymentName)
		// 	} else if args.Get.Pods.All{
		// 		docker.ListContainers("all")
		// 	}
		// case args.Remove != nil:
		// 	docker.RemoveContainers(args.Remove.Deployment)
	}

	return nil
}
