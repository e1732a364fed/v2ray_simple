package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ProjectName = "v2ray_simple"
	ProjectPath = ProjectName + "/"
)

func FileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

func DirExist(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}

// Function that search the specified file in the following directories:
//  -1. if starts with '/', or is an empty string, return directly
//  0. if starts with string similar to "C:/", "D:\\", or "e:/", return directly
//	1. Same folder with exec file
//  2. Same folder of the source file, 一种可能是用于 go test等情况
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
		//有可能是在项目子目录进行 go test的情况，此时可以试图在项目根目录寻找

		projectRootIdx := strings.Index(p, ProjectPath)
		if projectRootIdx >= 0 {
			p = p[:projectRootIdx+len(ProjectPath)] + fileName
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	if workingDir, err := os.Getwd(); err == nil {
		p := filepath.Join(workingDir, fileName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return fileName
}
