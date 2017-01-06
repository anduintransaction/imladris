package main

import (
	"path/filepath"
	"strings"
)

const (
	dataPath = "/mnt/sda1/var/data"
)

func translateFilePath(rootFolder, file string) string {
	if strings.HasPrefix(file, "/") || strings.HasPrefix(file, "~/") {
		return file
	}
	return filepath.Join(rootFolder, file)
}
