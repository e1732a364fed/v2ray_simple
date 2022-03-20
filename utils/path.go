package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

func FileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

// Function that search the specified file in the following directories:
//  0. if starts with '/', return directly
//	1. Same folder with exec file
//  2. Same folder of the source file, 应该是用于 go test等情况
//  3. Same folder of working folder
func GetFilePath(fileName string) string {

	if fileName[0] == '/' {
		return fileName
	}

	if execFile, err := os.Executable(); err == nil {

		p := filepath.Join(filepath.Dir(execFile), fileName)

		if _, err := os.Stat(p); err == nil {
			return p
		}

	}

	if _, srcFile, _, ok := runtime.Caller(0); ok {

		p := filepath.Join(filepath.Dir(srcFile), fileName)

		if _, err := os.Stat(p); err == nil {
			return p
		}

	}

	if workingDir, err := os.Getwd(); err == nil {

		p := filepath.Join(workingDir, fileName)

		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
