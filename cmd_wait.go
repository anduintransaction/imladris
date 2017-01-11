package main

import (
	"fmt"
	"os"

	"time"

	"k8s.io/client-go/1.4/pkg/api"
	v1batch "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/pkg/fields"
	"k8s.io/client-go/1.4/pkg/watch"
)

func cmdWait(args []string, config *appConfig) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "USAGE: %s jobname\n", os.Args[0])
		os.Exit(1)
	}
	jobName := args[0]
	namespace := "default"
	if config.namespace != "" {
		namespace = config.namespace
	}
	Printf(ColorYellow, "Waiting for job %q from namespace %q\n", jobName, namespace)
	clientset, err := loadKubernetesClient(config)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}

	job, err := clientset.Batch().Jobs(namespace).Get(jobName)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	checkJobStatus(job)
	selector, err := fields.ParseSelector("metadata.name=" + jobName)
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	watcher, err := clientset.Batch().Jobs(namespace).Watch(api.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		ErrPrintln(ColorRed, err)
		os.Exit(1)
	}
	timer := time.NewTimer(config.timeout)
	poller := time.NewTicker(time.Minute)
	pollErrorCount := 0
	for {
		var job *v1batch.Job
		select {
		case event := <-watcher.ResultChan():
			var ok bool
			job, ok = event.Object.(*v1batch.Job)
			if !ok {
				ErrPrintln(ColorRed, "Cannot decode job")
				os.Exit(1)
			}
			if event.Type == watch.Deleted {
				ErrPrintln(ColorRed, "Job was deleted")
				os.Exit(1)
			}
		case <-timer.C:
			ErrPrintln(ColorRed, "Timeout while waiting for job events")
			os.Exit(1)
		case <-poller.C:
			job, err = clientset.Batch().Jobs(namespace).Get(jobName)
			if err != nil {
				pollErrorCount++
				if pollErrorCount < 5 {
					continue
				}
				ErrPrintln(ColorRed, err)
				os.Exit(1)
			}
		}
		checkJobStatus(job)
	}
}

func checkJobStatus(job *v1batch.Job) {
	if len(job.Status.Conditions) > 0 {
		if job.Status.Conditions[0].Type == v1batch.JobComplete {
			Println(ColorGreen, "Job completed")
			os.Exit(0)
		} else {
			ErrPrintf(ColorRed, "Job failed: %s\n", job.Status.Conditions[0].Message)
			os.Exit(1)
		}
	}
}
