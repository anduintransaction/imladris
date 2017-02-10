package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"encoding/json"

	"strconv"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/apis/extensions/v1beta1"
)

type ContainerInfo struct {
	Name  string
	Image string
	Tag   string
}

type DeploymentInfo struct {
	Deployment *v1beta1.Deployment
	Containers map[string]*ContainerInfo
}

func getDeployment(kubeClient *kubernetes.Clientset, name, namespace string) (*DeploymentInfo, error) {
	deployment, err := kubeClient.Extensions().Deployments(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	deploymentInfo := &DeploymentInfo{
		Deployment: deployment,
		Containers: make(map[string]*ContainerInfo),
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		containerInfo := &ContainerInfo{
			Name: container.Name,
		}
		pieces := strings.Split(container.Image, ":")
		containerInfo.Image = pieces[0]
		if len(pieces) >= 2 {
			containerInfo.Tag = pieces[1]
		}
		deploymentInfo.Containers[containerInfo.Name] = containerInfo
	}
	return deploymentInfo, nil
}

func readAutoupdateCredential(rootFolder string, credential *AutoUpdateCredential) (string, string, error) {
	username := credential.Username
	password := credential.Password
	if password == "" {
		passwordFile := translateFilePath(rootFolder, credential.PasswordFile)
		buf, err := ioutil.ReadFile(passwordFile)
		if err != nil {
			return "", "", err
		}
		password = string(buf)
	}
	return username, password, nil
}

func findNewImageTag(image, username, password string) (string, error) {
	return findNewImageTagGCR(image, username, password)
}

func findNewImageTagGCR(image, username, password string) (string, error) {
	pieces := strings.Split(image, "/")
	if len(pieces) != 3 {
		return "", fmt.Errorf("invalid image: %q", image)
	}
	imageUser := pieces[1]
	imageName := pieces[2]
	url := "https://gcr.io/v2/" + imageUser + "/" + imageName + "/tags/list"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return "", err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	gcrTagList := &GCRTagList{}
	err = json.Unmarshal(content, gcrTagList)
	if err != nil {
		return "", err
	}
	latestTag := ""
	var latestTimestamp int64 = 0
	for _, manifest := range gcrTagList.Manifest {
		timestamp, err := strconv.ParseInt(manifest.TimeCreatedMs, 10, 64)
		if err != nil {
			return "", err
		}
		if timestamp <= latestTimestamp {
			continue
		}
		latestTimestamp = timestamp
		for _, tag := range manifest.Tag {
			if tag != "latest" {
				latestTag = tag
			}
		}
	}
	return latestTag, nil
}

type GCRTagList struct {
	Manifest map[string]GCRTagManifest `json:"manifest"`
}

type GCRTagManifest struct {
	Tag           []string `json:"tag"`
	TimeCreatedMs string   `json:"timeCreatedMs"`
}
