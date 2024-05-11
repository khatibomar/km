package main

import (
	"path/filepath"
	"strings"
)

func joinLinuxPath(elem ...string) string {
	joinedPath := filepath.Join(elem...)

	joinedPath = strings.ReplaceAll(joinedPath, "\\", "/")

	return joinedPath
}

func in(target string, list []string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
