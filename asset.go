package main

import (
	"strings"

	"bytes"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/apis/apps/v1alpha1"
	v1batch "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/pkg/apis/extensions/v1beta1"
	kubeyaml "k8s.io/client-go/1.4/pkg/util/yaml"
)

type UnsupportedResource string

func (err UnsupportedResource) Error() string {
	return "unsupported resource: " + string(err)
}

type Asset struct {
	Kind         string `yaml:"kind"`
	ResourceData interface{}
}

func parseAsset(data []byte) (*Asset, error) {
	asset := &Asset{}
	err := yaml.Unmarshal(data, asset)
	if err != nil {
		return nil, err
	}
	asset.Kind = strings.ToLower(asset.Kind)
	err = asset.parseResource(data)
	if err != nil {
		return nil, err
	}
	return asset, nil
}

func (asset *Asset) parseResource(data []byte) error {
	buf := bytes.NewReader(data)
	decoder := kubeyaml.NewYAMLOrJSONDecoder(buf, 1024)
	switch asset.Kind {
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
	case "petset":
		asset.ResourceData = &v1alpha1.PetSet{}
	default:
		return UnsupportedResource(asset.Kind)
	}
	err := decoder.Decode(asset.ResourceData)
	if err != nil {
		return err
	}
	return nil
}

type Meta interface {
	GetName() string
	GetNamespace() string
	SetNamespace(namespace string)
}
