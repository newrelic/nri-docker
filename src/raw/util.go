package raw

import (
	"errors"
	"os"
	"path"
	"path/filepath"
)

// returns a path that is located on the hostRoot folder of the host and the `/host` folder
// on the integrations. If they existed in both hostRoot and /host, returns the /host path,
// assuming the integration is running in a container
func containerToHost(hostFolder, hostPath string) string {
	insideContainerPath := path.Join(hostFolder, hostPath)
	var err error
	if _, err = os.Stat(insideContainerPath); err == nil {
		return insideContainerPath
	}
	return hostPath
}

var ErrHostRootNotFound = errors.New("no /proc folder found on the system")

func detectHostRoot(customHostRoot string, pathExists func(string) bool) (string, error) {
	if customHostRoot == "" {
		customHostRoot = "/host"
	}

	defaultHostRoot := "/"

	for _, hostRoot := range []string{customHostRoot, defaultHostRoot} {
		if pathExists(filepath.Join(hostRoot, "/proc")) {
			return hostRoot, nil
		}
	}

	return "", ErrHostRootNotFound
}
