// +build linux

package raw

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

var errHostRootNotFound = errors.New("no /proc folder found on the system")

type fileOpenFn func(string) (io.ReadCloser, error)

func defaultFileOpenFn(filePath string) (io.ReadCloser, error) {
	return os.Open(filePath)
}

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
