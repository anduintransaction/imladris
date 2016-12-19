package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os/exec"
	"time"
)

func checkDockerCommand() error {
	cmd := exec.Command("docker")
	return cmd.Run()
}

func dockerBuildImage(buildContext, tag string) error {
	Printf(ColorYellow, "Building docker image %q in %q\n", tag, buildContext)
	cmd := exec.Command("docker", "build", "-t", tag, buildContext)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return errors.New(errBuffer.String())
}

func dockerLogin(rootFolder string, credential *DockerCredential) error {
	host := credential.Host
	username := credential.Username
	password := credential.Password
	if password == "" {
		passwordFile := translateFilePath(rootFolder, credential.PasswordFile)
		buf, err := ioutil.ReadFile(passwordFile)
		if err != nil {
			return err
		}
		password = string(buf)
	}
	var cmd *exec.Cmd
	if host == "" {
		Println(ColorPurple, "Logging in to default docker registry")
		cmd = exec.Command("docker", "login", "-u", username, "-p", password)
	} else {
		Printf(ColorPurple, "Logging in to docker registry %q\n", host)
		cmd = exec.Command("docker", "login", "-u", username, "-p", password, host)
	}
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return errors.New(errBuffer.String())
}

func dockerRmi(name string) error {
	Printf(ColorYellow, "Auto clean image %s\n", name)
	var stdErr string
	for i := 0; i < 10; i++ {
		cmd := exec.Command("docker", "rmi", name)
		errBuffer := &bytes.Buffer{}
		cmd.Stderr = errBuffer
		err := cmd.Run()
		if err == nil {
			Printf(ColorGreen, "====> Success\n")
			return nil
		}
		stdErr = errBuffer.String()
		time.Sleep(5 * time.Second)
	}
	return errors.New(stdErr)
}

func dockerPush(name string) error {
	Printf(ColorYellow, "Pushing image %s\n", name)
	cmd := exec.Command("docker", "push", name)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	err := cmd.Run()
	if err == nil {
		return err
	}
	return errors.New(errBuffer.String())
}
