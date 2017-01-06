package main

import (
	"fmt"
	"os"
	"time"

	"io"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/fields"
	"k8s.io/client-go/1.4/pkg/watch"
)

func cmdLog(args []string, config *appConfig) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "USAGE: %s log pod-name\n", os.Args[0])
		os.Exit(1)
	}
	podName := args[0]
	namespace := "default"
	if config.namespace != "" {
		namespace = config.namespace
	}
	clientset, err := loadKubernetesClient(config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	pod, err := clientset.Core().Pods(namespace).Get(podName)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	if pod.Status.Phase == v1.PodUnknown {
		ErrPrintln(ColorRed, "Unknown pod status")
		os.Exit(1)
	}
	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		stream, err := getLogFromPod(clientset, namespace, podName, false)
		if err != nil {
			ErrPrintln(ColorRed, err)
			os.Exit(1)
		}
		defer stream.Close()
		io.Copy(os.Stdout, stream)
		if pod.Status.Phase == v1.PodSucceeded {
			os.Exit(0)
		}
		os.Exit(1)
	}
	selector, err := fields.ParseSelector("metadata.name=" + podName)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	watcher, err := clientset.Core().Pods(namespace).Watch(api.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	l := newLogFollower()
	err = l.followLog(clientset, namespace, podName)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	for {
		event := <-watcher.ResultChan()
		if event.Type == watch.Deleted {
			os.Exit(1)
		}
		watchedPod, ok := event.Object.(*v1.Pod)
		if !ok {
			ErrPrintln(ColorRed, "Cannot decode pod")
			os.Exit(1)
		}
		for _, status := range watchedPod.Status.ContainerStatuses {
			if status.State.Terminated != nil {
				switch watchedPod.Status.Phase {
				case v1.PodUnknown:
					ErrPrintln(ColorRed, "Unknown pod status")
					os.Exit(1)
				case v1.PodSucceeded:
					os.Exit(0)
				case v1.PodFailed:
					os.Exit(1)
				}
				l.close()
				break
			} else if status.State.Running != nil {
				l.start()
			}
		}
	}
}

type logFollowerEvent int

const (
	logFollowerEventStart logFollowerEvent = 1
	logFollowerEventClose logFollowerEvent = 2
)

type logFollower struct {
	eventChan chan logFollowerEvent
	lastEvent logFollowerEvent
}

func newLogFollower() *logFollower {
	return &logFollower{
		eventChan: make(chan logFollowerEvent),
		lastEvent: logFollowerEventStart,
	}
}

func (l *logFollower) start() {
	if l.lastEvent != logFollowerEventStart {
		l.lastEvent = logFollowerEventStart
		l.eventChan <- logFollowerEventStart
	}
}

func (l *logFollower) close() {
	if l.lastEvent != logFollowerEventClose {
		l.lastEvent = logFollowerEventClose
		l.eventChan <- logFollowerEventClose
	}
}

func (l *logFollower) followLog(kubeClient *kubernetes.Clientset, namespace, podName string) error {
	stream, err := getLogFromPod(kubeClient, namespace, podName, true)
	if err != nil {
		return err
	}
	go io.Copy(os.Stdout, stream)
	go func() {
		for {
			event := <-l.eventChan
			switch event {
			case logFollowerEventClose:
				// wait a bit
				time.Sleep(5 * time.Second)
				stream.Close()
			case logFollowerEventStart:
				stream, err = getLogFromPod(kubeClient, namespace, podName, true)
				if err != nil {
					ErrPrintln(ColorRed, err)
					os.Exit(1)
				}
				go io.Copy(os.Stdout, stream)
			}
		}
	}()
	return nil
}
