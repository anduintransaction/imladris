package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInteg(t *testing.T) {
	req := require.New(t)
	config := &appConfig{
		context:    "minikube",
		configFile: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
	}
	clientset, err := loadKubernetesClient(config)
	req.NoError(err)
	appRoot := "test-assets/integ"
	project, err := readProject(clientset, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	err = project.Up()
	req.NoError(err)
	ok, err := checkResourceExist(clientset, "deployment", "consul", "anduin")
	req.NoError(err)
	req.True(ok)
	ok, err = checkResourceExist(clientset, "service", "consul", "anduin")
	req.NoError(err)
	req.True(ok)
	ok, err = checkResourceExist(clientset, "job", "init", "anduin")
	req.NoError(err)
	req.True(ok)

	time.Sleep(5 * time.Second)

	err = project.Down()
	req.NoError(err)
	ok, err = checkResourceExist(clientset, "deployment", "consul", "anduin")
	req.NoError(err)
	req.False(ok)
	ok, err = checkResourceExist(clientset, "service", "consul", "anduin")
	req.NoError(err)
	req.False(ok)
	ok, err = checkResourceExist(clientset, "job", "init", "anduin")
	req.NoError(err)
	req.False(ok)
}
