package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/1.4/pkg/api/v1"
	v1batch "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/pkg/apis/extensions/v1beta1"
)

func TestDefaultVariables(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/simple"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	projectConfig := project.projectConfig
	req.Equal(projectConfig.Variables, map[string]string{
		"app_var_home":      os.Getenv("HOME"),
		"app_var_data_dir":  dataPath,
		"app_var_cwd":       "test-assets/config-tests/simple",
		"app_var_namespace": "default",
	})
}

func TestSimpleConfigError(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/simples"
	_, err := readProject(nil, appRoot, config)
	req.Error(err)
}

func TestSimpleConfigDefault(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/simple"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	projectConfig := project.projectConfig
	req.Equal(appRoot, projectConfig.RootFolder)
	req.Equal("default", projectConfig.Namespace)
	req.Len(project.services, 1)
	req.Len(project.resources, 0)
	req.Len(project.jobs, 0)
	service := project.services[0]
	req.Equal("deployment", service.Kind)
	deployment, ok := service.ResourceData.(*v1beta1.Deployment)
	req.True(ok)
	req.Equal("busybox", deployment.Name)
	req.Equal("default", deployment.Namespace)
	req.Len(deployment.Spec.Template.Spec.Containers, 1)
	req.Equal("busybox", deployment.Spec.Template.Spec.Containers[0].Name)
}

func TestSimpleConfigCustomNamespace(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/simple/deployments"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	projectConfig := project.projectConfig
	req.Equal("test-assets/config-tests/simple", projectConfig.RootFolder)
	req.Equal("anduin", projectConfig.Namespace)
	req.Len(project.services, 1)
	service := project.services[0]
	deployment, ok := service.ResourceData.(*v1beta1.Deployment)
	req.True(ok)
	req.Equal("anduin", deployment.Namespace)
}

func TestSimpleConfigNamespaceFromVariable(t *testing.T) {
	req := require.New(t)
	config := &appConfig{
		namespace: "anduin-dep",
	}
	appRoot := "test-assets/config-tests/simple/deployments/dep.yml"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	projectConfig := project.projectConfig
	req.Equal("test-assets/config-tests/simple", projectConfig.RootFolder)
	req.Equal("anduin-dep", projectConfig.Namespace)
	req.Len(project.services, 1)
	service := project.services[0]
	deployment, ok := service.ResourceData.(*v1beta1.Deployment)
	req.True(ok)
	req.Equal("anduin-dep", deployment.Namespace)
}

func TestSimpleConfigBuild(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/simple/deployments/build.yml"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)
	projectConfig := project.projectConfig
	req.Len(projectConfig.Build, 2)
	req.Equal(projectConfig.Build[0], &ProjectBuild{
		Name: "anduin/test",
		Tag:  "1.2.1",
		From: "build/test",
	})
	req.Equal(projectConfig.Build[1], &ProjectBuild{
		Name:    "anduin/test2",
		Tag:     "3.1.4",
		VarName: "test_image_2",
		From:    "build/test2",
	})
	req.Equal(projectConfig.Variables["build_var_anduin_test"], "anduin/test:1.2.1")
	req.Equal(projectConfig.Variables["test_image_2"], "anduin/test2:3.1.4")
}

func TestConfigNotSimpleLocal(t *testing.T) {
	req := require.New(t)
	config := &appConfig{
		variables: map[string]string{
			"variable_common_tag": "2.4.8",
		},
	}
	appRoot := "test-assets/config-tests/not-simple/deployments/local.yml"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)

	// Check resources
	req.Len(project.resources, 1)
	checkNotSimpleResourceConfig(req, project.resources[0])

	// Check jobs
	checkNotSimpleJobs(req, project.jobs)

	// Check services
	req.Len(project.services, 2)
	checkNotSimpleServiceCommon(req, project.services[0], "2.4.8")
	checkNotSimpleServiceLocal(req, project.services[1])
}

func TestConfigNotSimpleRemote(t *testing.T) {
	req := require.New(t)
	config := &appConfig{}
	appRoot := "test-assets/config-tests/not-simple/deployments/remote.yml"
	project, err := readProject(nil, appRoot, config)
	req.NoError(err)
	req.NotNil(project)

	// Check resources
	req.Len(project.resources, 2)
	checkNotSimpleResourceConfig(req, project.resources[0])
	checkNotSimpleResourceDB(req, project.resources[1])

	// Check jobs
	checkNotSimpleJobs(req, project.jobs)

	// Check services
	req.Len(project.services, 2)
	checkNotSimpleServiceCommon(req, project.services[0], "1.2.4")
	checkNotSimpleServiceRemote(req, project.services[1])
}

func checkNotSimpleResourceConfig(req *require.Assertions, resource *Asset) {
	req.Equal("configmap", resource.Kind)
	configMaps, ok := resource.ResourceData.(*v1.ConfigMap)
	req.True(ok)
	req.Equal("config", configMaps.Name)
	req.Equal("value", configMaps.Data["key"])
}

func checkNotSimpleResourceDB(req *require.Assertions, resource *Asset) {
	req.Equal("persistentvolumeclaim", resource.Kind)
	pvc, ok := resource.ResourceData.(*v1.PersistentVolumeClaim)
	req.True(ok)
	req.Equal("db", pvc.Name)
}

func checkNotSimpleJobs(req *require.Assertions, jobs []*Asset) {
	req.Len(jobs, 1)
	req.Equal("job", jobs[0].Kind)
	job, ok := jobs[0].ResourceData.(*v1batch.Job)
	req.True(ok)
	req.Equal("init", job.Name)
	req.Len(job.Spec.Template.Spec.Containers, 1)
	req.Equal("anduin/initimage:1.4.3", job.Spec.Template.Spec.Containers[0].Image)
}

func checkNotSimpleServiceCommon(req *require.Assertions, service *Asset, tag string) {
	req.Equal("deployment", service.Kind)
	deployment, ok := service.ResourceData.(*v1beta1.Deployment)
	req.True(ok)
	req.Equal("common", deployment.Name)
	req.Len(deployment.Spec.Template.Spec.Containers, 1)
	req.Equal("common:"+tag, deployment.Spec.Template.Spec.Containers[0].Image)
}

func checkNotSimpleServiceLocal(req *require.Assertions, service *Asset) {
	req.Equal("service", service.Kind)
	srv, ok := service.ResourceData.(*v1.Service)
	req.True(ok)
	req.Equal("local", srv.Name)
}

func checkNotSimpleServiceRemote(req *require.Assertions, service *Asset) {
	req.Equal("service", service.Kind)
	srv, ok := service.ResourceData.(*v1.Service)
	req.True(ok)
	req.Equal("remote", srv.Name)
}
