package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/1.4/kubernetes"
)

type Project struct {
	kubeClient    *kubernetes.Clientset
	projectConfig *ProjectConfig
	projectFolder string
	resources     []*Asset
	services      []*Asset
	jobs          []*Asset
	excludes      map[string]struct{}
}

type ProjectConfig struct {
	RootFolder            string                  `yaml:"root_folder"`
	Pulls                 []string                `yaml:"pulls"`
	InitUp                []string                `yaml:"init_up"`
	InitDown              []string                `yaml:"init_down"`
	FinalizeUp            []string                `yaml:"finalize_up"`
	FinalizeDown          []string                `yaml:"finalize_down"`
	Services              []string                `yaml:"services"`
	Jobs                  []string                `yaml:"jobs"`
	Resources             []string                `yaml:"resources"`
	Excludes              []string                `yaml:"excludes"`
	Namespace             string                  `yaml:"namespace"`
	Variables             map[string]string       `yaml:"variables"`
	Build                 []*ProjectBuild         `yaml:"build"`
	Credentials           []*DockerCredential     `yaml:"credentials"`
	DeleteNamespace       bool                    `yaml:"delete_namespace"`
	AutoUpdates           []*AutoUpdate           `yaml:"auto_updates"`
	AutoUpdateCredentials []*AutoUpdateCredential `yaml:"auto_update_credentials"`
}

type ProjectBuild struct {
	Name       string `yaml:"name"`
	VarName    string `yaml:"var_name"`
	Tag        string `yaml:"tag"`
	From       string `yaml:"from"`
	Push       bool   `yaml:"push"`
	PushLatest bool   `yaml:"push_latest"`
	AutoClean  bool   `yaml:"auto_clean"`
}

type DockerCredential struct {
	Host         string `yaml:"host"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

type AutoUpdate struct {
	Name       string                 `yaml:"name"`
	Containers []*AutoUpdateContainer `yaml:"containers"`
}

type AutoUpdateContainer struct {
	Name       string `yaml:"name"`
	Credential string `yaml:"credential"`
}

type AutoUpdateCredential struct {
	Name         string `yaml:"name"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

func readProject(kubeClient *kubernetes.Clientset, assetRoot string, config *appConfig) (*Project, error) {
	p := &Project{
		kubeClient:    kubeClient,
		projectConfig: &ProjectConfig{},
	}
	var err error
	if err != nil {
		return nil, err
	}
	err = p.readProjectConfig(assetRoot, config.variables)
	if err != nil {
		return nil, err
	}
	if config.namespace != "" {
		p.projectConfig.Namespace = config.namespace
	}
	if p.projectConfig.Namespace == "" {
		p.projectConfig.Namespace = "default"
	}
	if p.projectConfig.RootFolder != "" {
		p.projectConfig.RootFolder = translateFilePath(p.projectFolder, p.projectConfig.RootFolder)
	} else {
		p.projectConfig.RootFolder = p.projectFolder
	}
	if p.projectConfig.Variables == nil {
		p.projectConfig.Variables = make(map[string]string)
	}
	for key, value := range config.variables {
		p.projectConfig.Variables[key] = value
	}
	p.projectConfig.Variables["app_var_namespace"] = p.projectConfig.Namespace
	p.projectConfig.Variables["app_var_home"] = os.Getenv("HOME")
	p.projectConfig.Variables["app_var_data_dir"] = dataPath
	p.projectConfig.Variables["app_var_cwd"] = p.projectConfig.RootFolder

	// Read build info
	err = p.readBuild()
	if err != nil {
		return nil, err
	}

	// Read excludes
	err = p.readExcludes()
	if err != nil {
		return nil, err
	}

	// Read assets
	p.resources, err = p.readAssets(p.projectConfig.RootFolder, p.projectConfig.Resources, "resources/*")
	if err != nil {
		return nil, err
	}
	p.jobs, err = p.readAssets(p.projectConfig.RootFolder, p.projectConfig.Jobs, "jobs/*")
	if err != nil {
		return nil, err
	}
	p.services, err = p.readAssets(p.projectConfig.RootFolder, p.projectConfig.Services, "services/*")
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) readProjectConfig(assetRoot string, variables variableMap) error {
	projectFile := assetRoot
	p.projectFolder = filepath.Dir(assetRoot)
	info, err := os.Stat(projectFile)
	if err != nil && os.IsNotExist(err) {
		return err
	}
	if err != nil || info.IsDir() {
		projectFile = assetRoot + "/project.yml"
		p.projectFolder = assetRoot
		_, err := os.Stat(projectFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			return nil
		}
	}
	data, err := ioutil.ReadFile(projectFile)
	if err != nil {
		return err
	}
	t, err := template.New(projectFile).Parse(string(data))
	if err != nil {
		return err
	}
	t = t.Option("missingkey=error")
	buf := &bytes.Buffer{}
	err = t.Execute(buf, variables)
	if err != nil {
		return err
	}
	projectConfig := &ProjectConfig{}
	err = yaml.Unmarshal(buf.Bytes(), projectConfig)
	if err != nil {
		return err
	}
	p.projectConfig = projectConfig
	return nil
}

func (p *Project) readBuild() error {
	invalidChar := regexp.MustCompile("[^a-zA-Z0-9_]")
	underscores := regexp.MustCompile("_+")
	for _, build := range p.projectConfig.Build {
		varName := build.VarName
		if varName == "" {
			varName = "build_var_" + underscores.ReplaceAllString(invalidChar.ReplaceAllString(build.Name, "_"), "_")
		}
		tagName := build.Name + ":" + build.Tag
		p.projectConfig.Variables[varName] = tagName
	}
	return nil
}

func (p *Project) readExcludes() error {
	p.excludes = make(map[string]struct{})
	for _, glob := range p.projectConfig.Excludes {
		glob = translateFilePath(p.projectConfig.RootFolder, glob)
		excludes, err := filepath.Glob(glob)
		if err != nil {
			return err
		}
		for _, exclude := range excludes {
			p.excludes[exclude] = struct{}{}
		}
	}
	return nil
}

func (p *Project) readAssets(rootFolder string, globs []string, defaultGlob string) ([]*Asset, error) {
	if len(globs) == 0 {
		globs = []string{defaultGlob}
	}
	assetFiles := []string{}
	for _, glob := range globs {
		glob = translateFilePath(rootFolder, glob)
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		assetFiles = append(assetFiles, matches...)
	}
	assets := []*Asset{}
	for _, filename := range assetFiles {
		_, ok := p.excludes[filename]
		if ok {
			continue
		}
		asset, err := p.readAsset(filename)
		if err != nil {
			return nil, err
		}
		if asset != nil {
			assets = append(assets, asset)
		}
	}
	return assets, nil
}

func (p *Project) readAsset(filename string) (*Asset, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, nil
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filename).Parse(string(data))
	if err != nil {
		return nil, err
	}
	t = t.Option("missingkey=error")
	buf := &bytes.Buffer{}
	err = t.Execute(buf, p.projectConfig.Variables)
	if err != nil {
		return nil, err
	}
	asset, err := parseAsset(buf.Bytes())
	if err != nil {
		return nil, err
	}
	asset.UpdateNamespace(p.projectConfig.Namespace)
	return asset, nil
}

func (p *Project) runScripts(scripts []string) error {
	for _, script := range scripts {
		Printf(ColorYellow, "Running script %q\n", script)
		cmd := exec.Command("sh", "-c", script)
		cmd.Dir = p.projectConfig.RootFolder
		output, err := cmd.CombinedOutput()
		fmt.Print(string(output))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) dockerLogin() error {
	for _, credential := range p.projectConfig.Credentials {
		err := dockerLogin(p.projectConfig.RootFolder, credential)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) Up() error {
	if len(p.projectConfig.Pulls) > 0 {
		err := p.pullImages()
		if err != nil {
			return err
		}
	}
	err := p.runScripts(p.projectConfig.InitUp)
	if err != nil {
		return err
	}
	err = p.dockerLogin()
	if err != nil {
		return err
	}
	err = p.build()
	if err != nil {
		return err
	}
	err = createNamespace(p.kubeClient, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	for _, resource := range p.resources {
		err := p.createAsset(resource)
		if err != nil {
			return err
		}
	}
	for _, job := range p.jobs {
		err := p.createAsset(job)
		if err != nil {
			return err
		}
	}
	for _, service := range p.services {
		err := p.createAsset(service)
		if err != nil {
			return err
		}
	}
	return p.runScripts(p.projectConfig.FinalizeUp)
}

func (p *Project) pullImages() error {
	imagesToPull := make(map[string]struct{})
	for _, imageName := range p.projectConfig.Pulls {
		imagesToPull[imageName] = struct{}{}
	}
	for _, resource := range p.resources {
		err := p.pullImage(resource, imagesToPull)
		if err != nil {
			return err
		}
	}
	for _, job := range p.jobs {
		err := p.pullImage(job, imagesToPull)
		if err != nil {
			return err
		}
	}
	for _, service := range p.services {
		err := p.pullImage(service, imagesToPull)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) pullImage(asset *Asset, imagesToPull map[string]struct{}) error {
	images, err := getResourceImages(asset.Kind, asset.ResourceData)
	if err != nil {
		return err
	}
	for _, image := range images {
		pieces := strings.Split(image, ":")
		imageName := pieces[0]
		_, ok := imagesToPull[imageName]
		if ok {
			err = dockerPull(image)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Project) build() error {
	for _, build := range p.projectConfig.Build {
		err := p.buildDockerImage(build)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) buildDockerImage(build *ProjectBuild) error {
	buildContext := translateFilePath(p.projectConfig.RootFolder, build.From)
	tagName := build.Name + ":" + build.Tag
	err := dockerBuildImage(buildContext, tagName)
	if err != nil {
		return err
	}
	if !build.Push {
		return nil
	}
	return dockerPush(tagName, build.PushLatest)
}

func (p *Project) Down() error {
	err := p.runScripts(p.projectConfig.InitDown)
	if err != nil {
		return err
	}
	for _, service := range p.services {
		err := p.destroyAsset(service)
		if err != nil {
			return err
		}
	}
	for _, job := range p.jobs {
		err := p.destroyAsset(job)
		if err != nil {
			return err
		}
	}
	for _, resource := range p.resources {
		err := p.destroyAsset(resource)
		if err != nil {
			return err
		}
	}
	if p.projectConfig.DeleteNamespace {
		err = deleteNamespace(p.kubeClient, p.projectConfig.Namespace)
		if err != nil {
			return err
		}
	}
	for _, build := range p.projectConfig.Build {
		if build.AutoClean {
			err = dockerRmi(build.Name + ":" + build.Tag)
			if err != nil {
				// Bail error here
				ErrPrintln(ColorRed, err)
			}
			if build.Push && build.PushLatest {
				err = dockerRmi(build.Name + ":latest")
				if err != nil {
					// Also Bail error here
					ErrPrintln(ColorRed, err)
				}
			}
		}
	}
	return p.runScripts(p.projectConfig.FinalizeDown)
}

func (p *Project) createAsset(asset *Asset) error {
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	Printf(ColorYellow, "Creating %s %q from namespace %q\n", asset.Kind, assetName, p.projectConfig.Namespace)
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	if existed {
		Println(ColorGreen, "====> Existed")
		return nil
	}
	err = createResource(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace, asset.ResourceData)
	if err == nil {
		Println(ColorGreen, "====> Success")
	}
	return err
}

func (p *Project) destroyAsset(asset *Asset) error {
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	Printf(ColorYellow, "Destroying %s %q from namespace %q\n", asset.Kind, assetName, p.projectConfig.Namespace)
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	if !existed && asset.Kind != "pod" {
		Println(ColorGreen, "====> Not existed")
		return nil
	}
	err = destroyResource(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace)
	if err == nil {
		Println(ColorGreen, "====> Success")
	}
	return err
}

func (p *Project) Update() error {
	if len(p.projectConfig.Pulls) > 0 {
		err := p.pullImages()
		if err != nil {
			return err
		}
	}
	err := p.runScripts(p.projectConfig.InitUp)
	if err != nil {
		return err
	}
	err = p.dockerLogin()
	if err != nil {
		return err
	}
	err = p.build()
	if err != nil {
		return err
	}
	err = createNamespace(p.kubeClient, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	for _, resource := range p.resources {
		err := p.updateAsset(resource)
		if err != nil {
			return err
		}
	}
	for _, job := range p.jobs {
		err := p.updateAsset(job)
		if err != nil {
			return err
		}
	}
	for _, service := range p.services {
		err := p.updateAsset(service)
		if err != nil {
			return err
		}
	}
	return p.runScripts(p.projectConfig.FinalizeUp)
}

func (p *Project) updateAsset(asset *Asset) error {
	if asset.Kind != "pod" && asset.Kind != "deployment" && asset.Kind != "configmap" && asset.Kind != "secret" {
		return nil
	}
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	Printf(ColorYellow, "Updating %s %q from namespace %q\n", asset.Kind, assetName, p.projectConfig.Namespace)
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	if !existed {
		Println(ColorGreen, "====> Not existed")
		return nil
	}
	err = updateResource(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace, asset.ResourceData)
	if err == nil {
		Println(ColorGreen, "====> Success")
	}
	return err
}

func (p *Project) AutoUpdate(version string) error {
	if version == "" {
		Println(ColorYellow, "Will automatically search for latest version")
	} else {
		Printf(ColorYellow, "Autoupdate to %s\n", version)
	}
	autoUpdates := make(map[string]*AutoUpdate)
	for _, autoUpdate := range p.projectConfig.AutoUpdates {
		autoUpdates[autoUpdate.Name] = autoUpdate
	}
	credentials := make(map[string]*AutoUpdateCredential)
	for _, credential := range p.projectConfig.AutoUpdateCredentials {
		credentials[credential.Name] = credential
	}
	for _, resource := range p.resources {
		err := p.autoupdateAsset(resource, autoUpdates, credentials, version)
		if err != nil {
			return err
		}
	}
	for _, job := range p.jobs {
		err := p.autoupdateAsset(job, autoUpdates, credentials, version)
		if err != nil {
			return err
		}
	}
	for _, service := range p.services {
		err := p.autoupdateAsset(service, autoUpdates, credentials, version)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) autoupdateAsset(asset *Asset, autoUpdates map[string]*AutoUpdate, autoUpdateCredentials map[string]*AutoUpdateCredential, newTag string) error {
	if asset.Kind != "deployment" {
		return nil
	}
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	autoUpdateInfo, ok := autoUpdates[assetName]
	if !ok {
		return nil
	}
	Printf(ColorYellow, "Autoupdate %s %q from namespace %q\n", asset.Kind, assetName, p.projectConfig.Namespace)
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	if !existed {
		Println(ColorGreen, "====> Not existed")
		return nil
	}
	deploymentInfo, err := getDeployment(p.kubeClient, assetName, p.projectConfig.Namespace)
	if err != nil {
		return err
	}
	newContainers := make(map[string]string)
	for _, containerInfo := range autoUpdateInfo.Containers {
		oldContainer := deploymentInfo.Containers[containerInfo.Name]
		if oldContainer == nil {
			ErrPrintf(ColorRed, "====> Container not found: %q\n", containerInfo.Name)
			continue
		}
		// We only support gcr.io at the moment
		if !strings.HasPrefix(oldContainer.Image, "gcr.io") && newTag == "" {
			ErrPrintf(ColorPurple, "====> We only support gcr.io at the moment, skipping container %q (%q)\n", containerInfo.Name, oldContainer.Image)
			return nil
		}
		if newTag == "" {
			credential := autoUpdateCredentials[containerInfo.Credential]
			var username, password string
			if credential != nil {
				username, password, err = readAutoupdateCredential(p.projectConfig.RootFolder, credential)
				if err != nil {
					return err
				}
			}
			newTag, err = findNewImageTag(oldContainer.Image, username, password)
			if err != nil {
				return err
			}
		}
		if newTag == oldContainer.Tag {
			Printf(ColorPurple, "====> Same tag %q, skipping container %q (%q)\n", containerInfo.Name, oldContainer.Image)
			continue
		}
		newContainers[oldContainer.Name] = oldContainer.Image + ":" + newTag
	}
	if len(newContainers) == 0 {
		Println(ColorGreen, "====> No new container found")
		return nil
	}
	for i, container := range deploymentInfo.Deployment.Spec.Template.Spec.Containers {
		newImage, ok := newContainers[container.Name]
		if ok {
			container.Image = newImage
			deploymentInfo.Deployment.Spec.Template.Spec.Containers[i] = container
		}
	}
	_, err = p.kubeClient.Extensions().Deployments(p.projectConfig.Namespace).Update(deploymentInfo.Deployment)
	if err == nil {
		Printf(ColorGreen, "====> Updated deployment %q:\n", assetName)
		for containerName, newImage := range newContainers {
			Printf(ColorGreen, "====> %q to %q\n", containerName, newImage)
		}
	}
	return err
}
