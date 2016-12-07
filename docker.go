package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type DockerBuildResponse struct {
	Stream string `json:"string"`
	Error  string `json:"error"`
}

func dockerBuildImage(dockerClient *client.Client, buildContext string, tag string) error {
	fmt.Printf("Building %q from %q\n", tag, buildContext)
	buf := &bytes.Buffer{}
	err := dockerPackageBuildContext(buildContext, buf)
	if err != nil {
		return err
	}
	response, err := dockerClient.ImageBuild(context.Background(), buf, types.ImageBuildOptions{
		Tags: []string{tag},
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		dockerBuildResponse := &DockerBuildResponse{}
		err = json.Unmarshal([]byte(line), dockerBuildResponse)
		if err != nil {
			return err
		}
		if dockerBuildResponse.Error != "" {
			return errors.New(dockerBuildResponse.Error)
		}
	}
	return scanner.Err()
}

func dockerPackageBuildContext(buildContext string, buf io.Writer) error {
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	err := filepath.Walk(buildContext, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		relativePath := strings.Trim(strings.TrimPrefix(path, buildContext), "/")
		if relativePath == "" {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relativePath
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, r)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
