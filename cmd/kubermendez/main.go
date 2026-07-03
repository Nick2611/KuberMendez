package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/alexflint/go-arg"
	"kuberMendez/deployment-parser"
)

type ApplyCMD struct {
	File string `arg:"-f, required" help:"Apply a given deployment document"`
}

type ValidateCMD struct {
	File string `arg:"-f, required" help:"Validate a given deployment document, will not take any effect on the deployment itself"`
}

type args struct{
	Apply *ApplyCMD			`arg:"subcommand:apply, positional" help:"Used to create deployments"`
	Validate *ValidateCMD	`arg:"subcommand:validate" help:"Used to validate deployments before applying them"`
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
	}
}