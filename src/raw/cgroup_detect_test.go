package raw

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

func createFileOpenFnMock(filesMap map[string]string) func(string) (io.ReadCloser, error) {
	return func(filePath string) (io.ReadCloser, error) {
		if fileContent, ok := filesMap[filePath]; ok {
			return ioutil.NopCloser(strings.NewReader(fileContent)), nil
		}

		return nil, fmt.Errorf("file not found by path: %s", filePath)
	}
}
