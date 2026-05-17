//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func DataDir() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(base, "coachly")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
