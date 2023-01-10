package main

import (
	"fmt"
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
