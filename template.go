package main

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func makePath(segments ...string) (string, error) {
	if len(segments) > 0 && strings.HasPrefix(segments[0], "/") {
		return filepath.Join(segments...), nil
	}
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
