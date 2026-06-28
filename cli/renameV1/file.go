package main

import (
	"os"
	"path/filepath"
	"sort"
)

func ListDirFiles(dirPath string, postfix []string) ([]string, error) {
	var fileList []string

	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	dirContent, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	for _, f := range dirContent {
		if !f.IsDir() && filterFileName(f.Name(), postfix) {
			fileList = append(fileList, f.Name())
		}
	}

	sort.Strings(fileList)
	return fileList, nil
}

func filterFileName(fileName string, postfixSlice []string) bool {
	if len(postfixSlice) == 0 {
		return true
	}
	for _, postfix := range postfixSlice {
		if fileName[len(fileName)-len(postfix):] == postfix {
			return true
		}
	}
	return false
}

func FileExist(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func Join(dir, file string) string {
	abs, _ := filepath.Abs(dir)

	return filepath.Join(abs, file)
}
