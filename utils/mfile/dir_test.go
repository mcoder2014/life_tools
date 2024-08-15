package mfile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListDirFiles(t *testing.T) {
	t.Run("ListDirFiles", func(t *testing.T) {
		files, err := ListDirFiles("/tmp")
		require.NoError(t, err)
		for i, file := range files {
			t.Logf("idx:%v\tfile:%v", i, file)
		}
	})

	t.Run("ListDir", func(t *testing.T) {
		files, dirs, err := ListDir("/tmp")
		require.NoError(t, err)
		for i, file := range files {
			t.Logf("idx:%v\tfile:%v", i, file)
		}
		fmt.Printf("\n\n")
		for i, dir := range dirs {
			t.Logf("idx:%v\tdir:%v", i, dir)
		}
	})

	t.Run("RecursiveAllFiles", func(t *testing.T) {
		allFiles, err := RecursiveAllFiles("/tmp", false)
		require.NoError(t, err)
		for i, file := range allFiles {
			t.Logf("idx:%v\tfile:%v", i, file)
		}
	})
}
