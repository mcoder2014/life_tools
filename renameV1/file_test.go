package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListDirFiles(t *testing.T) {
	var testDir = "./testData"

	fileList, err := ListDirFiles(testDir, []string{".mkv", ".mp4"})
	require.NoError(t, err)
	for _, fileName := range fileList {
		fmt.Println(fileName)
	}
}

func TestRenameFileNameWithSpace(t *testing.T) {

	f, err := os.Create("./testData/file name with space.mkv")
	require.NoError(t, err)
	f.Write([]byte("test"))
	err = f.Close()
	require.NoError(t, err)

	err = os.Rename("./testData/file name with space.mkv", "./testData/file_name_with_space.mkv")
	require.NoError(t, err)
	err = os.Remove("./testData/file_name_with_space.mkv")
	require.NoError(t, err)
}
