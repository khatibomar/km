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
