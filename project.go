package main

import (
	"fmt"
	"os"

	"io/ioutil"

	"path/filepath"

	"strings"

	"bytes"
	"text/template"

	"regexp"

	"os/exec"

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
}

type ProjectConfig struct {
	RootFolder   string            `yaml:"root_folder"`
	InitUp       []string          `yaml:"init_up"`
	InitDown     []string          `yaml:"init_down"`
	FinalizeUp   []string          `yaml:"finalize_up"`
	FinalizeDown []string          `yaml:"finalize_down"`
	Services     []string          `yaml:"services"`
	Jobs         []string          `yaml:"jobs"`
	Resources    []string          `yaml:"resources"`
	Namespace    string            `yaml:"namespace"`
	Variables    map[string]string `yaml:"variables"`
	Build        []*ProjectBuild   `yaml:"build"`
}

type ProjectBuild struct {
	Name    string `yaml:"name"`
	VarName string `yaml:"var_name"`
	Tag     string `yaml:"tag"`
	From    string `yaml:"from"`
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
	if p.projectConfig.RootFolder != "" {
		if !strings.HasPrefix(p.projectConfig.RootFolder, "/") && !strings.HasPrefix(p.projectConfig.RootFolder, "~/") {
			p.projectConfig.RootFolder = filepath.Join(p.projectFolder, p.projectConfig.RootFolder)
		}
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
	p.projectConfig.Variables["app_data_dir"] = "/data"
	// Read build info
	err = p.readBuild()
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
	if info.IsDir() || err != nil {
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

func (p *Project) readAssets(rootFolder string, globs []string, defaultGlob string) ([]*Asset, error) {
	if len(globs) == 0 {
		globs = []string{defaultGlob}
	}
	assetFiles := []string{}
	for _, glob := range globs {
		if !strings.HasPrefix(glob, "/") && !strings.HasPrefix(glob, "~/") {
			glob = filepath.Join(rootFolder, glob)
			matches, err := filepath.Glob(glob)
			if err != nil {
				return nil, err
			}
			assetFiles = append(assetFiles, matches...)
		}
	}
	assets := []*Asset{}
	for _, filename := range assetFiles {
		asset, err := p.readAsset(filename)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func (p *Project) readAsset(filename string) (*Asset, error) {
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
	return parseAsset(buf.Bytes())
}

func (p *Project) runScripts(scripts []string) error {
	for _, script := range scripts {
		if !strings.HasPrefix(script, "/") && !strings.HasPrefix(script, "~/") {
			script = filepath.Join(p.projectConfig.RootFolder, script)
		}
		cmd := exec.Command("sh", "-c", script)
		output, err := cmd.CombinedOutput()
		fmt.Print(string(output))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) Up() error {
	err := p.runScripts(p.projectConfig.InitUp)
	if err != nil {
		return err
	}
	err = p.build()
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
	buildContext := build.From
	if !strings.HasPrefix(buildContext, "/") && !strings.HasPrefix(buildContext, "~/") {
		buildContext = filepath.Join(p.projectConfig.RootFolder, buildContext)
	}
	return dockerBuildImage(buildContext, build.Name+":"+build.Tag)
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
	return p.runScripts(p.projectConfig.FinalizeDown)
}

func (p *Project) createAsset(asset *Asset) error {
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	assetNamespace := objectMeta.GetNamespace()
	if p.projectConfig.Namespace != "" {
		assetNamespace = p.projectConfig.Namespace
	}
	fmt.Printf("Creating %s %q from namespace %q\n", asset.Kind, assetName, assetNamespace)
	objectMeta.SetNamespace(assetNamespace)
	err := createNamespace(p.kubeClient, assetNamespace)
	if err != nil {
		return err
	}
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, assetNamespace)
	if err != nil {
		return err
	}
	if existed {
		return nil
	}
	return createResource(p.kubeClient, asset.Kind, assetNamespace, asset.ResourceData)
}

func (p *Project) destroyAsset(asset *Asset) error {
	objectMeta := asset.ResourceData.(Meta)
	assetName := objectMeta.GetName()
	assetNamespace := objectMeta.GetNamespace()
	if p.projectConfig.Namespace != "" {
		assetNamespace = p.projectConfig.Namespace
	}
	fmt.Printf("Destroying %s %q from namespace %q\n", asset.Kind, assetName, assetNamespace)
	existed, err := checkResourceExist(p.kubeClient, asset.Kind, assetName, assetNamespace)
	if err != nil {
		return err
	}
	if !existed {
		return nil
	}
	return destroyResource(p.kubeClient, asset.Kind, assetName, assetNamespace)
}
