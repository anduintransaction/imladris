//go:generate go-bindata -o templates/generated.go -pkg templates templates/files
package main

import (
	"io/ioutil"
	"os"

	"github.com/anduintransaction/imladris/templates"
)

func cmdGenerate(args []string, config *appConfig) {
	if len(args) < 2 {
		ErrPrintf(ColorWhite, "Usage: %s generate [project|pod|deployment|service|job|persistentvolumeclaim|configmap] filename\n", os.Args[0])
		os.Exit(1)
	}
	templateName := args[0]
	filename := args[1]
	switch args[0] {
	case "project", "pod", "deployment", "service", "job", "persistentvolumeclaim", "configmap":
		asset, err := templates.Asset("templates/files/" + templateName + ".yml")
		if err != nil {
			ErrPrintln(ColorRed, err)
			os.Exit(1)
		}
		if err != nil {
			ErrPrintln(ColorRed, err)
			os.Exit(1)
		}
		err = ioutil.WriteFile(filename, asset, os.FileMode(0644))
		if err != nil {
			ErrPrintln(ColorRed, err)
			os.Exit(1)
		}
	default:
		ErrPrintf(ColorWhite, "Usage: %s generate [project|pod|deployment|service|job|persistentvolumeclaim|configmap] filename\n", os.Args[0])
		os.Exit(1)
	}
}
