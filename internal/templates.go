package internal

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
)

func GetTemplateContent(filename string) (string, error) {
	// load path relative to calling source file
	_, callerFile, _, _ := runtime.Caller(0) //nolint:dogsled
	rootDir := filepath.Dir(callerFile)
	absRootDir := rootDir[:len(rootDir)-8]
	content, err := ioutil.ReadFile(path.Join(absRootDir, "internal", "templates", filename))
	if err != nil {
		return "", fmt.Errorf("could not read template file: %v", err)
	}
	return string(content), nil
}
