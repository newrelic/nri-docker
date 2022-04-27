package raw

import (
	"io"
	"os"
)

const mountsFilePath = "/proc/mounts"

type fileOpenFn func(string) (io.ReadCloser, error)

func defaultFileOpenFn(filePath string) (io.ReadCloser, error) {
	return os.Open(filePath)
}
