package raw

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// Gets uint64 parsed content of single value cgroup stat file
// This func has been extracted from utils.go on the cgroup repo since is unexported
func ParseStatFileContentUint64(filePath string) (uint64, error) {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "max" {
		return math.MaxUint64, nil
	}

	res, err := parseUint(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse %q as a uint from Cgroup file %q", string(contents), filePath)
	}

	return res, nil
}

func parseUint(s string, base, bitSize int) (uint64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. Handle negative values greater than MinInt64 (and)
		// 2. Handle negative values lesser than MinInt64
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil &&
			intErr.(*strconv.NumError).Err == strconv.ErrRange &&
			intValue < 0 {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}
