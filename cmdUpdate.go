package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

func downloadBinary(url string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	gzBuffer, err := gzip.NewReader(response.Body)
	if err != nil {
		return err
	}
	defer gzBuffer.Close()
	tarBuffer := tar.NewReader(gzBuffer)
	binaryFile, err := os.Create("/usr/local/bin/anduin-minikube-deploy")
	defer binaryFile.Close()
	if err != nil {
		return err
	}
	_, err = tarBuffer.Next()
	if err != nil {
		return err
	}
	_, err = io.Copy(binaryFile, tarBuffer)
	return err
}

func cmdUpdate(args []string, config *appConfig) {
	if len(args) < 1 {
		ErrPrintf(ColorWhite, "Usage: %s update version\n", os.Args[0])
		os.Exit(1)
	}
	version := args[0]
	Printf(ColorYellow, "Downloading anduin-minikube-deploy-%s-%s\n", version, runtime.GOOS)
	url := fmt.Sprintf("https://github.com/anduintransaction/anduin-minikube-deploy/releases/download/%s/anduin-minikube-deploy-%s-%s-amd64.tar.gz", version, version, runtime.GOOS)
	err := downloadBinary(url)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
}
