package main

import (
	"errors"
	"fmt"
	"os/exec"
)

import "bytes"

type DockerBuildResponse struct {
	Stream string `json:"string"`
	Error  string `json:"error"`
}

func checkDockerCommand() error {
	cmd := exec.Command("docker")
	return cmd.Run()
}

func dockerBuildImage(buildContext, tag string) error {
	fmt.Printf("Building docker image %q in %q\n", tag, buildContext)
	cmd := exec.Command("docker", "build", "-t", tag, buildContext)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return errors.New(errBuffer.String())
}
