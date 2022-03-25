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
//  -1. if starts with '/', return directly
//  0. if starts with things similar to "C:/" or "D:\\", return directly
//	1. Same folder with exec file
//  2. Same folder of the source file, 应该是用于 go test等情况
//  3. Same folder of working folder
func GetFilePath(fileName string) string {
	if len(fileName) < 1 {
		return ""
	}

	fb := fileName[0]
	if fb == '/' && len(fileName) > 1 {
		return fileName
	}

	if len(fileName) > 3 && (fb >= 'C' && fb <= 'Z' || fb >= 'c' && fb <= 'z') && fileName[1] == ':' && (fileName[2] == '/' || fileName[2] == '\\') {
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
