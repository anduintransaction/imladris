package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	v1batch "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
)

func loadKubernetesClient(config *appConfig) (*kubernetes.Clientset, error) {
	clientConfigLoader := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: config.configFile,
	}
	configOverrides := &clientcmd.ConfigOverrides{}
	if config.context != "" {
		configOverrides.CurrentContext = config.context
	}
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientConfigLoader, configOverrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeConfig)
}

type KubernetesResource struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

func loadKubernetesResource(data []byte) (*KubernetesResource, error) {
	r := &KubernetesResource{}
	err := yaml.Unmarshal(data, r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createNamespace(kubeClient *kubernetes.Clientset, namespace string) error {
	_, err := kubeClient.Core().Namespaces().Get(namespace)
	if err == nil {
		return nil
	}
	if !isResourceNotExist(err) {
		return err
	}
	ns := &v1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err = kubeClient.Core().Namespaces().Create(ns)
	return err
}

func deleteNamespace(kubeClient *kubernetes.Clientset, namespace string) error {
	if namespace == "default" {
		return nil
	}
	_, err := kubeClient.Core().Namespaces().Get(namespace)
	if err != nil {
		if isResourceNotExist(err) {
			return nil
		}
		return err
	}
	for i := 0; i < 10; i++ {
		err = kubeClient.Core().Namespaces().Delete(namespace, &v1.DeleteOptions{})
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return err
	}
	return nil
}

func checkResourceExist(kubeClient *kubernetes.Clientset, kind, name, namespace string) (bool, error) {
	var err error
	switch kind {
	case "pod":
		pod, err := kubeClient.Core().Pods(namespace).Get(name)
		if err != nil {
			if isResourceNotExist(err) {
				return false, nil
			}
			return false, err
		}
		switch pod.Status.Phase {
		case v1.PodUnknown:
			return false, fmt.Errorf("unknown pod status")
		case v1.PodSucceeded, v1.PodFailed:
			return false, nil
		default:
			return true, nil
		}
	case "deployment":
		_, err = kubeClient.Extensions().Deployments(namespace).Get(name)
	case "service":
		_, err = kubeClient.Core().Services(namespace).Get(name)
	case "job":
		_, err = kubeClient.Batch().Jobs(namespace).Get(name)
	case "persistentvolumeclaim":
		_, err = kubeClient.Core().PersistentVolumeClaims(namespace).Get(name)
	case "configmap":
		_, err = kubeClient.Core().ConfigMaps(namespace).Get(name)
	case "secret":
		_, err = kubeClient.Core().Secrets(namespace).Get(name)
	case "ingress":
		_, err = kubeClient.Extensions().Ingresses(namespace).Get(name)
	case "endpoints":
		_, err = kubeClient.Core().Endpoints(namespace).Get(name)
	case "daemonset":
		_, err = kubeClient.Extensions().DaemonSets(namespace).Get(name)
	default:
		return false, UnsupportedResource(kind)
	}
	if err == nil {
		return true, nil
	}
	if isResourceNotExist(err) {
		return false, nil
	}
	return false, err
}

func createResource(kubeClient *kubernetes.Clientset, kind, name, namespace string, resourceData interface{}) error {
	var err error
	retry := 0
	for {
		switch kind {
		case "pod":
			// Delete if possible
			deleteOptions := v1.NewDeleteOptions(0)
			kubeClient.Core().Pods(namespace).Delete(name, deleteOptions)
			_, err = kubeClient.Core().Pods(namespace).Create(resourceData.(*v1.Pod))
		case "deployment":
			_, err = kubeClient.Extensions().Deployments(namespace).Create(resourceData.(*v1beta1.Deployment))
		case "service":
			_, err = kubeClient.Core().Services(namespace).Create(resourceData.(*v1.Service))
		case "job":
			_, err = kubeClient.Batch().Jobs(namespace).Create(resourceData.(*v1batch.Job))
		case "persistentvolumeclaim":
			_, err = kubeClient.Core().PersistentVolumeClaims(namespace).Create(resourceData.(*v1.PersistentVolumeClaim))
		case "configmap":
			_, err = kubeClient.Core().ConfigMaps(namespace).Create(resourceData.(*v1.ConfigMap))
		case "secret":
			_, err = kubeClient.Core().Secrets(namespace).Create(resourceData.(*v1.Secret))
		case "ingress":
			_, err = kubeClient.Extensions().Ingresses(namespace).Create(resourceData.(*v1beta1.Ingress))
		case "endpoints":
			_, err = kubeClient.Core().Endpoints(namespace).Create(resourceData.(*v1.Endpoints))
		case "daemonset":
			_, err = kubeClient.Extensions().DaemonSets(namespace).Create(resourceData.(*v1beta1.DaemonSet))
		default:
			return UnsupportedResource(kind)
		}
		if err == nil {
			return nil
		}
		statusErr, ok := err.(*errors.StatusError)
		if !ok {
			return err
		}
		message := statusErr.Status().Message
		if strings.Contains(message, "unable to create new content in namespace") && strings.Contains(message, "being terminated") {
			retry++
			if retry > 10 {
				return err
			}
			time.Sleep(5 * time.Second)
			continue
		} else if strings.Contains(message, "namespaces") && strings.Contains(message, "not found") {
			err = createNamespace(kubeClient, namespace)
			if err != nil {
				return err
			}
		} else {
			return err
		}

	}
	return err
}

func destroyResource(kubeClient *kubernetes.Clientset, kind, name, namespace string) error {
	var err error
	deleteOptions := v1.NewDeleteOptions(0)
	switch kind {
	case "pod":
		return destroyPod(kubeClient, name, namespace)
	case "deployment":
		return destroyDeployment(kubeClient, name, namespace)
	case "service":
		err = kubeClient.Core().Services(namespace).Delete(name, deleteOptions)
	case "job":
		return destroyJob(kubeClient, name, namespace)
	case "persistentvolumeclaim":
		err = kubeClient.Core().PersistentVolumeClaims(namespace).Delete(name, deleteOptions)
	case "configmap":
		err = kubeClient.Core().ConfigMaps(namespace).Delete(name, deleteOptions)
	case "secret":
		err = kubeClient.Core().Secrets(namespace).Delete(name, deleteOptions)
	case "ingress":
		err = kubeClient.Extensions().Ingresses(namespace).Delete(name, deleteOptions)
	case "endpoints":
		err = kubeClient.Core().Endpoints(namespace).Delete(name, deleteOptions)
	case "daemonset":
		err = kubeClient.Extensions().DaemonSets(namespace).Delete(name, deleteOptions)
	default:
		return UnsupportedResource(kind)
	}
	return err
}

func updateResource(kubeClient *kubernetes.Clientset, kind, name, namespace string, resourceData interface{}) error {
	var err error
	switch kind {
	case "pod":
		_, err = kubeClient.Core().Pods(namespace).Update(resourceData.(*v1.Pod))
	case "deployment":
		_, err = kubeClient.Extensions().Deployments(namespace).Update(resourceData.(*v1beta1.Deployment))
	case "service":
		return nil
	case "job":
		return nil
	case "persistentvolumeclaim":
		return nil
	case "configmap":
		_, err = kubeClient.Core().ConfigMaps(namespace).Update(resourceData.(*v1.ConfigMap))
	case "secret":
		_, err = kubeClient.Core().Secrets(namespace).Update(resourceData.(*v1.Secret))
	case "ingress":
		_, err = kubeClient.Extensions().Ingresses(namespace).Update(resourceData.(*v1beta1.Ingress))
	case "endpoints":
		_, err = kubeClient.Core().Endpoints(namespace).Update(resourceData.(*v1.Endpoints))
	case "daemonset":
		_, err = kubeClient.Extensions().DaemonSets(namespace).Update(resourceData.(*v1beta1.DaemonSet))
	default:
		return UnsupportedResource(kind)
	}
	return err
}

func getResourceImages(kind string, resourceData interface{}) ([]string, error) {
	var containers []v1.Container
	switch kind {
	case "pod":
		containers = resourceData.(*v1.Pod).Spec.Containers
	case "deployment":
		containers = resourceData.(*v1beta1.Deployment).Spec.Template.Spec.Containers
	case "job":
		containers = resourceData.(*v1batch.Job).Spec.Template.Spec.Containers
	case "service", "persistentvolumeclaim", "configmap", "secret", "ingress":
		return nil, nil
	default:
		return nil, UnsupportedResource(kind)
	}
	images := []string{}
	for _, container := range containers {
		images = append(images, container.Image)
	}
	return images, nil
}

func destroyPod(kubeClient *kubernetes.Clientset, name, namespace string) error {
	deleteOptions := v1.NewDeleteOptions(0)
	err := kubeClient.Core().Pods(namespace).Delete(name, deleteOptions)
	if err == nil {
		return nil
	}
	statusErr, ok := err.(*errors.StatusError)
	if !ok {
		return err
	}
	if statusErr.Status().Code == 404 {
		return nil
	}
	return err
}

func destroyDeployment(kubeClient *kubernetes.Clientset, name, namespace string) error {
	deleteOptions := v1.NewDeleteOptions(0)
	err := kubeClient.Extensions().Deployments(namespace).Delete(name, deleteOptions)
	if err != nil {
		return err
	}
	listOptions := v1.ListOptions{
		LabelSelector: "name=" + name,
	}
	err = kubeClient.Extensions().ReplicaSets(namespace).DeleteCollection(deleteOptions, listOptions)
	if err != nil {
		return err
	}
	return kubeClient.Core().Pods(namespace).DeleteCollection(deleteOptions, listOptions)
}

func destroyJob(kubeClient *kubernetes.Clientset, name, namespace string) error {
	deleteOptions := v1.NewDeleteOptions(0)
	err := kubeClient.Batch().Jobs(namespace).Delete(name, deleteOptions)
	if err != nil {
		return err
	}
	pods, err := kubeClient.Core().Pods(namespace).List(v1.ListOptions{
		LabelSelector: "job-name=" + name,
	})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err = kubeClient.Core().Pods(namespace).Delete(pod.Name, deleteOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

func getLogFromPod(kubeClient *kubernetes.Clientset, namespace, podName string, follow bool) (io.ReadCloser, error) {
	var stream io.ReadCloser
	var err error
	for {
		stream, err = kubeClient.Core().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
			Follow: follow,
		}).Stream()
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "ContainerCreating") {
			time.Sleep(time.Second)
		} else {
			return nil, err
		}
	}
	return stream, nil
}

func getEvents(kubeClient *kubernetes.Clientset, namespace, podName string) ([]v1.Event, error) {
	events, err := kubeClient.Core().Events(namespace).List(v1.ListOptions{
		FieldSelector: "involvedObject.name=" + podName,
	})
	if err != nil {
		return nil, err
	}
	return events.Items, err
}

func getLastEvent(kubeClient *kubernetes.Clientset, namespace, podName string) (*v1.Event, error) {
	events, err := getEvents(kubeClient, namespace, podName)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("no event found")
	}
	return &events[len(events)-1], nil
}

func isResourceNotExist(err error) bool {
	switch err := err.(type) {
	case *errors.StatusError:
		if err.Status().Code == 404 {
			return true
		}
		return false
	default:
		return false
	}
}
