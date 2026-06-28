package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTailWriterKeepsLastBytes(t *testing.T) {
	w := newTailWriter(5)

	n, err := w.Write([]byte("abc"))
	require.NoError(t, err)
	require.Equal(t, 3, n)

	n, err = w.Write([]byte("defgh"))
	require.NoError(t, err)
	require.Equal(t, 5, n)

	require.Equal(t, "defgh", w.String())
}

func TestTailWriterKeepsLastBytesForLargeWrite(t *testing.T) {
	w := newTailWriter(4)

	n, err := w.Write([]byte("0123456789"))
	require.NoError(t, err)
	require.Equal(t, 10, n)

	require.Equal(t, "6789", w.String())
}

func TestCommandExecutorCapturesAndPassesThroughStdoutAndStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exec := commandExecutorWithIO([]string{"sh", "-c", "printf 'visible-out'; printf 'boom-err' >&2; exit 7"}, nil, &stdout, &stderr)

	result := exec(context.Background())

	require.Error(t, result.Err)
	require.Equal(t, 7, result.ExitCode)
	require.Contains(t, result.StdoutTail, "visible-out")
	require.Contains(t, result.StderrTail, "boom-err")
	require.Contains(t, stdout.String(), "visible-out")
	require.Contains(t, stderr.String(), "boom-err")
}

func TestCommandOutputTailPrefersStderr(t *testing.T) {
	msg := commandOutputTail(commandResult{
		StdoutTail: "stdout noise",
		StderrTail: "stderr failure",
	})

	require.Contains(t, msg, "stderr")
	require.Contains(t, msg, "stderr failure")
	require.NotContains(t, msg, "stdout noise")
}

func TestCommandOutputTailFallsBackToStdout(t *testing.T) {
	msg := commandOutputTail(commandResult{
		StdoutTail: "stdout failure",
	})

	require.Contains(t, msg, "stdout")
	require.Contains(t, msg, "stdout failure")
}

func TestCommandOutputTailSkipsEmptyOutput(t *testing.T) {
	msg := commandOutputTail(commandResult{})

	require.Equal(t, "", strings.TrimSpace(msg))
}
