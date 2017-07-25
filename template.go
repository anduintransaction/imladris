package main

import (
	"os"
	"path/filepath"
	"text/template"
)

func makePath(segments ...string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	paths := append([]string{pwd}, segments...)
	return filepath.Join(paths...), nil
}

func getFuncMap() template.FuncMap {
	return template.FuncMap{
		"makePath": makePath,
	}
}
