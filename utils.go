package main

import (
	"path/filepath"
	"strings"
)

func translateFilePath(rootFolder, file string) string {
	if strings.HasPrefix(file, "/") || strings.HasPrefix(file, "~/") {
		return file
	}
	return filepath.Join(rootFolder, file)
}
