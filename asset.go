package main

import (
	"fmt"
	"strings"

	"bytes"

	"gopkg.in/yaml.v2"
	v1batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type UnsupportedResource string

func (err UnsupportedResource) Error() string {
	return "unsupported resource: " + string(err)
}

type Asset struct {
	Kind         string `yaml:"kind"`
	ResourceData interface{}
	filename     string
	data         []byte
}

func parseAsset(filename string, data []byte) (*Asset, error) {
	asset := &Asset{}
	asset.filename = filename
	asset.data = data
	err := yaml.Unmarshal(data, asset)
	if err != nil {
		return nil, fmt.Errorf("unable to parse asset %q, error: %s", asset.filename, err.Error())
	}
	asset.Kind = strings.ToLower(asset.Kind)
	err = asset.parseResource(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse asset %q, error: %s", asset.filename, err.Error())
	}
	return asset, nil
}

func (asset *Asset) parseResource(data []byte) error {
	buf := bytes.NewReader(data)
	decoder := kubeyaml.NewYAMLOrJSONDecoder(buf, 1024)
	switch asset.Kind {
	case "pod":
		asset.ResourceData = &v1.Pod{}
	case "deployment":
		asset.ResourceData = &v1beta1.Deployment{}
	case "service":
		asset.ResourceData = &v1.Service{}
	case "job":
		asset.ResourceData = &v1batch.Job{}
	case "persistentvolumeclaim":
		asset.ResourceData = &v1.PersistentVolumeClaim{}
	case "configmap":
		asset.ResourceData = &v1.ConfigMap{}
	case "secret":
		asset.ResourceData = &v1.Secret{}
	case "ingress":
		asset.ResourceData = &v1beta1.Ingress{}
	case "endpoints":
		asset.ResourceData = &v1.Endpoints{}
	case "daemonset":
		asset.ResourceData = &v1beta1.DaemonSet{}
	default:
		return UnsupportedResource(asset.Kind)
	}
	err := decoder.Decode(asset.ResourceData)
	if err != nil {
		return err
	}
	return nil
}

func (asset *Asset) UpdateNamespace(namespace string) {
	objectMeta := asset.ResourceData.(Meta)
	objectMeta.SetNamespace(namespace)
}

func (asset *Asset) Debug() {
	fmt.Println(string(asset.data))
}

type Meta interface {
	GetName() string
	GetNamespace() string
	SetNamespace(namespace string)
}
