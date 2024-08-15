package mfile

import (
	"io/ioutil"
	"os"
	"path"
)

// ListDirFiles 获取指定路径下的所有文件，只搜索当前路径
// input /tmp
// output /tmp/file1.txt
func ListDirFiles(dir string) (files []string, err error) {
	files = []string{}

	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, fileinfo := range contents {
		if fileinfo.IsDir() {
			continue //忽略目录
		}
		files = append(files, path.Join(dir, fileinfo.Name()))
	}

	return files, nil
}

func ListDir(dirPath string) ([]string, []string, error) {
	var fileList []string
	var dirList []string

	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()

	dirContent, err := dir.Readdir(-1)
	if err != nil {
		return nil, nil, err
	}

	for _, f := range dirContent {
		p := path.Join(dirPath, f.Name())
		if f.IsDir() {
			dirList = append(dirList, p)
		} else {
			fileList = append(fileList, p)
		}
	}
	return fileList, dirList, nil
}

// RecursiveAllFiles 递归获取目录下所有文件，包含递归子路径下的文件
func RecursiveAllFiles(dirPath string, skipErr bool) ([]string, error) {
	var fileList []string
	var dirList []string

	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	dirContents, err := dir.Readdir(-1)
	if err != nil {
		_ = dir.Close()
		return nil, err
	}
	// 用完即关
	_ = dir.Close()
	for _, f := range dirContents {
		p := path.Join(dirPath, f.Name())
		if f.IsDir() {
			dirList = append(dirList, p)
		} else {
			fileList = append(fileList, p)
		}
	}

	for _, subDirPath := range dirList {
		contents, err := RecursiveAllFiles(subDirPath, skipErr)
		if err != nil && !skipErr {
			return nil, err
		}
		fileList = append(fileList, contents...)
	}
	return fileList, nil
}
