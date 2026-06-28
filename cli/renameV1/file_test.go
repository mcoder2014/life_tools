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
	const tmpDir = "./tmp"

	require.NoError(t, os.RemoveAll(tmpDir))
	require.NoError(t, os.MkdirAll(tmpDir, 0755))
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	oldName := tmpDir + "/file name with space.mkv"
	newName := tmpDir + "/file_name_with_space.mkv"
	f, err := os.Create(oldName)
	require.NoError(t, err)
	_, err = f.Write([]byte("test"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, os.Rename(oldName, newName))
	_, err = os.Stat(newName)
	require.NoError(t, err)
}
