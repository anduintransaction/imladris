package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

func checkDockerCommand() error {
	cmd := exec.Command("docker")
	return cmd.Run()
}

func dockerBuildImage(buildContext, tag string) error {
	Printf(ColorYellow, "Building docker image %q in %q\n", tag, buildContext)
	cmd := exec.Command("docker", "build", "-t", tag, buildContext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	return errors.New("cannot build docker image")
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
	for i := 0; i < 20; i++ {
		cmd := exec.Command("docker", "rmi", name)
		errBuffer := &bytes.Buffer{}
		cmd.Stderr = errBuffer
		err := cmd.Run()
		if err == nil {
			Printf(ColorGreen, "====> Success\n")
			return nil
		}
		stdErr = errBuffer.String()
		if strings.Contains(stdErr, "must force") {
			time.Sleep(5 * time.Second)
			continue
		} else {
			return errors.New(stdErr)
		}
	}
	return errors.New(stdErr)
}

func dockerImageExistLocally(name string) bool {
	cmd := exec.Command("docker", "images", name)
	buff := &bytes.Buffer{}
	cmd.Stdout = buff
	cmd.Run()
	pieces := strings.Split(name, ":")
	if strings.Contains(buff.String(), pieces[0]) {
		return true
	}
	return false
}

func dockerPull(name string) error {
	if dockerImageExistLocally(name) {
		return nil
	}
	Printf(ColorYellow, "Pulling image %s\n", name)
	cmd := exec.Command("docker", "pull", name)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err == nil {
		return err
	}
	return errors.New(errBuffer.String())
}

func dockerPush(name string, pushLatest bool) error {
	err := doDockerPush(name)
	if err != nil {
		return err
	}
	if !pushLatest {
		return nil
	}
	pieces := strings.Split(name, ":")
	imageName := pieces[0]
	latestImage := imageName + ":latest"
	err = dockerTag(name, latestImage)
	if err != nil {
		return err
	}
	return doDockerPush(latestImage)
}

func doDockerPush(name string) error {
	Printf(ColorYellow, "Pushing image %s\n", name)
	cmd := exec.Command("docker", "push", name)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return errors.New(errBuffer.String())
	}
	return nil
}

func dockerTag(name, alias string) error {
	Printf(ColorYellow, "Tagging %q as %q", name, alias)
	cmd := exec.Command("docker", "tag", name, alias)
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return errors.New(errBuffer.String())
	}
	return nil
}
