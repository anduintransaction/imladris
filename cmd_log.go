package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	tail := "-1"
	for {
		tailPodLog(clientset, podName, namespace, config.context, tail)

		// Check pod exit Status
		waitExit := 0
	wait_exit:
		for {
			pod, err := clientset.Core().Pods(namespace).Get(podName, apiv1.GetOptions{})
			if err != nil {
				ErrPrintln(ColorRed, err)
				os.Exit(1)
			}

			switch pod.Status.Phase {
			case v1.PodSucceeded:
				os.Exit(0)
			case v1.PodFailed:
				ErrPrintln(ColorRed, "Pod failed: ", pod.Status.ContainerStatuses[0].State.Terminated.Reason)
				os.Exit(1)
			case v1.PodRunning, v1.PodPending:
				waitExit++
				if waitExit <= 5 {
					time.Sleep(time.Second)
				} else {
					containerStatus := pod.Status.ContainerStatuses[0]
					fmt.Println(containerStatus)
					if containerStatus.State.Terminated != nil {
						os.Exit(int(containerStatus.State.Terminated.ExitCode))
					}
					ErrPrintln(ColorRed, "Log stream quited while pod still running")
					ErrPrintln(ColorRed, "Restart log stream, some log lines may be lost")
					waitExit = 0
					tail = "1"
					break wait_exit
				}
			default:
				ErrPrintln(ColorRed, "Unknown pod phase")
				os.Exit(1)
			}
		}
	}

}

func tailPodLog(clientset *kubernetes.Clientset, podName, namespace, context, tail string) {
	// Wait for pod running
wait_running:
	for {
		pod, err := clientset.Core().Pods(namespace).Get(podName, apiv1.GetOptions{})
		if err != nil {
			ErrPrintln(ColorRed, err)
			os.Exit(1)
		}
		switch pod.Status.Phase {
		case v1.PodUnknown:
			ErrPrintln(ColorRed, "Unknown pod phase")
			os.Exit(1)
		case v1.PodPending:
			time.Sleep(2 * time.Second)
		default:
			break wait_running
		}
	}
	var cmd *exec.Cmd
	if context == "" {
		cmd = exec.Command("kubectl", "logs", "-f", "--tail", tail, podName, "--namespace", namespace)
	} else {
		cmd = exec.Command("kubectl", "logs", "-f", "--tail", tail, podName, "--namespace", namespace, "--context", context)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
}
