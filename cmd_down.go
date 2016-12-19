package main

import "os"

func cmdDown(args []string, config *appConfig) {
	clientset, err := loadKubernetesClient(config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	assetRoot := "."
	if len(args) > 0 {
		assetRoot = args[0]
	}
	project, err := readProject(clientset, assetRoot, config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	err = project.Down()
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
}
