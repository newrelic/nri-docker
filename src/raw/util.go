package raw

import (
	"errors"
	"os"
	"path/filepath"
)

var errHostRootNotFound = errors.New("no /proc folder found on the system")

// DetectHostRoot returns a path that is located on the hostRoot folder of the host and the `/host` folder
// on the integrations. If they existed in both hostRoot and /host, returns the /host path,
// assuming the integration is running in a container
func DetectHostRoot(hostRoot string, pathExists func(string) bool) (string, error) {
	if hostRoot == "" {
		hostRoot = "/host"
	}

	defaultHostRoot := "/"

	for _, hostRoot := range []string{hostRoot, defaultHostRoot} {
		if pathExists(filepath.Join(hostRoot, "/proc")) {
			return hostRoot, nil
		}
	}

	return "", errHostRootNotFound
}

// CanAccessDir returns true if the the dir is accessible.
func CanAccessDir(dir string) bool {
	_, err := os.Stat(dir)

	return err == nil
}
