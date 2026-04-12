package apppath

import (
	"os"
	"path/filepath"
	"runtime"
)

func Path() string {
	path := ""
	switch runtime.GOOS {
	case "windows":
		path = filepath.Join(os.Getenv("APPDATA"), "YuLiReBa")
	case "darwin":
		path = "/Library/Application Support/YuLiReBa"
	default:
		path = filepath.Join(os.Getenv("HOME"), ".YuLiReBa")
	}
	_ = os.MkdirAll(path, 0644)
	return path
}
