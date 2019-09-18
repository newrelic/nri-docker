package paths

import (
	"os"
	"path"
)

// returns a path that is located on the root folder of the host and the `/host` folder
// on the integrations. If they existed in both root and /host, returns the /host path,
// assuming the integration is running in a container
func ContainerToHost(hostFolder, hostPath string) string {
	insideContainerPath := path.Join(hostFolder, hostPath)
	var err error
	if _, err = os.Stat(insideContainerPath); err == nil {
		return insideContainerPath
	}
	return hostPath
}
