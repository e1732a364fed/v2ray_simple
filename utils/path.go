package utils

import (
	"errors"
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

//判断一个字符串是否是合法的文件名, 注意本函数不实际检查是否存在该文件
func IsFilePath(s string) error {

	//https://stackoverflow.com/questions/1976007/what-characters-are-forbidden-in-windows-and-linux-directory-names

	if runtime.GOOS == "windows" {
		if strings.ContainsAny(s, ":<>\"/\\|?*") {
			return errors.New("contain illegal characters")
		}
		if strings.ContainsAny(s, string([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31})) {
			return errors.New("contain illegal ASCII control characters")
		}
	} else {
		if strings.Contains(s, string([]byte{0})) {
			return errors.New("contain illegal characters")
		}
	}
	return nil
}

// Function that search the specified file in the following directories:
//  -1. if starts with '/', or is an empty string, return directly
//  0. if starts with string similar to "C:/", "D:\\", or "e:/", return directly
//	1. Same folder with exec file
//  2. Same folder of the source file, 一种可能 是 用于 go test等情况
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

	// 下面的 runtime.Caller 查找调用函数对应的源文件所在目录。只适合普通go build的情况，如果是release版，因为使用了 trimpath参数，
	// 就找不到源文件的路径了。不过一般使用发布版的人 也不是 git clone的，而是直接下载的，所以本来也找不到, 所以无所谓。

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
