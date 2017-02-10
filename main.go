package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type appConfig struct {
	configFile string
	context    string
	namespace  string
	timeout    time.Duration
	variables  variableMap
}

type variableMap map[string]string

func (v *variableMap) String() string {
	return fmt.Sprint(*v)
}

func (v *variableMap) Set(value string) error {
	pieces := strings.SplitN(value, "=", 2)
	if len(pieces) != 2 {
		return nil
	}
	(*v)[pieces[0]] = pieces[1]
	return nil
}

func main() {
	// check docker command
	err := checkDockerCommand()
	if err != nil {
		ErrPrintln(ColorRed, "docker command not found, please install docker command line")
		os.Exit(1)
	}
	config := &appConfig{
		variables: make(variableMap),
	}
	flag.StringVar(&config.configFile, "kubeconfig", "", "Kube config file")
	flag.StringVar(&config.context, "context", "", "Kube context")
	flag.StringVar(&config.namespace, "namespace", "", "Kube namespace")
	flag.DurationVar(&config.timeout, "timeout", 15*time.Minute, "timeout duration")
	flag.Var(&config.variables, "variable", "override variables")
	flag.Parse()

	if config.configFile == "" {
		config.configFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
	}
	switch args[0] {
	case "version":
		cmdVersion(args[1:], config)
	case "up":
		cmdUp(args[1:], config)
	case "down":
		cmdDown(args[1:], config)
	case "update":
		cmdUpdate(args[1:], config)
	case "wait":
		cmdWait(args[1:], config)
	case "log":
		cmdLog(args[1:], config)
	case "data":
		cmdData(args[1:], config)
	case "generate":
		cmdGenerate(args[1:], config)
	case "autoupdate":
		cmdAutoUpdate(args[1:], config)
	default:
		printUsage()
	}
}

func printUsage() {
	ErrPrintf(ColorWhite, "USAGE: %s <flag> [command] <folder>\n", os.Args[0])
	ErrPrintf(ColorWhite, "Available commands: up, down, update, version, wait, log, data, generate\n")
	flag.PrintDefaults()
	os.Exit(2)
}
