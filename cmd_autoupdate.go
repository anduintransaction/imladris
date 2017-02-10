package main

import "os"

func cmdAutoUpdate(args []string, config *appConfig) {
	clientset, err := loadKubernetesClient(config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	assetRoot := "."
	if len(args) > 0 {
		assetRoot = args[0]
	}
	newVersion := ""
	if len(args) > 1 {
		newVersion = args[1]
	}
	project, err := readProject(clientset, assetRoot, config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	err = project.AutoUpdate(newVersion)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
}
