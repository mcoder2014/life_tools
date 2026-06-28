package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerShowsRootAndDirectoryListing(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	require.NoError(t, os.Mkdir(shareDir, 0755))
	writeTestFile(t, filepath.Join(shareDir, "visible.txt"), "visible")
	writeTestFile(t, filepath.Join(shareDir, ".env"), "secret")
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir, Name: "Public"})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	body := getBody(t, server.URL+"/")
	require.Contains(t, body, "Public")
	require.Contains(t, body, "/browse/0")

	body = getBody(t, server.URL+"/browse/0/")
	require.Contains(t, body, "visible.txt")
	require.Contains(t, body, ".env")
	require.Contains(t, body, "/raw/0/visible.txt")
	require.Contains(t, body, "/download/0/visible.txt")
	require.Contains(t, body, "/zip/0/")
}

func TestServerEscapesSpecialNamesInLinks(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	require.NoError(t, os.Mkdir(shareDir, 0755))
	writeTestFile(t, filepath.Join(shareDir, "a #?.txt"), "special")
	writeTestFile(t, filepath.Join(shareDir, "a%2Fb.txt"), "percent")
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	body := getBody(t, server.URL+"/browse/0/")
	require.Contains(t, body, "/raw/0/a%20%23%3F.txt")
	require.Contains(t, body, "/raw/0/a%252Fb.txt")

	body = getBody(t, server.URL+"/raw/0/a%20%23%3F.txt")
	require.Equal(t, "special", body)
	body = getBody(t, server.URL+"/raw/0/a%252Fb.txt")
	require.Equal(t, "percent", body)
}

func TestServerServesInlineAndAttachment(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	writeTestFile(t, file, "hello file")
	entries := mustBuildEntries(t, ConfigEntry{Path: file})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	resp := getResponse(t, http.MethodGet, server.URL+"/raw/0/")
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "inline", resp.Header.Get("Content-Disposition"))
	require.Equal(t, "hello file", string(content))

	resp = getResponse(t, http.MethodGet, server.URL+"/download/0/")
	defer resp.Body.Close()
	content, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	require.Contains(t, resp.Header.Get("Content-Disposition"), "note.txt")
	require.Equal(t, "hello file", string(content))
}

func TestServerRejectsEscapingShareRoot(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	require.NoError(t, os.Mkdir(shareDir, 0755))
	writeTestFile(t, filepath.Join(root, "secret.txt"), "secret")
	writeTestFile(t, filepath.Join(shareDir, "visible.txt"), "visible")
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	for _, path := range []string{
		"/raw/0/../secret.txt",
		"/raw/0/%2e%2e/secret.txt",
		"/raw/0/../visible.txt",
	} {
		resp := getResponse(t, http.MethodGet, server.URL+path)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusNotFound, resp.StatusCode, path)
	}
}

func TestServerAllowsSymlinkTargets(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	targetDir := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(shareDir, 0755))
	require.NoError(t, os.Mkdir(targetDir, 0755))
	writeTestFile(t, filepath.Join(targetDir, "outside.txt"), "outside")
	require.NoError(t, os.Symlink(filepath.Join(targetDir, "outside.txt"), filepath.Join(shareDir, "link.txt")))
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	body := getBody(t, server.URL+"/raw/0/link.txt")

	require.Equal(t, "outside", body)
}

func TestServerFollowsSymlinkDirectories(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	targetDir := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(shareDir, 0755))
	require.NoError(t, os.Mkdir(targetDir, 0755))
	writeTestFile(t, filepath.Join(targetDir, "outside.txt"), "outside")
	require.NoError(t, os.Symlink(targetDir, filepath.Join(shareDir, "linked-dir")))
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	body := getBody(t, server.URL+"/browse/0/")
	require.Contains(t, body, "/browse/0/linked-dir")

	body = getBody(t, server.URL+"/browse/0/linked-dir")
	require.Contains(t, body, "outside.txt")

	body = getBody(t, server.URL+"/raw/0/linked-dir/outside.txt")
	require.Equal(t, "outside", body)
}

func TestServerDownloadsDirectoryZip(t *testing.T) {
	root := t.TempDir()
	shareDir := filepath.Join(root, "share")
	subDir := filepath.Join(shareDir, "sub")
	targetDir := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.Mkdir(targetDir, 0755))
	writeTestFile(t, filepath.Join(shareDir, "a.txt"), "a")
	writeTestFile(t, filepath.Join(subDir, "b.txt"), "b")
	writeTestFile(t, filepath.Join(targetDir, "linked.txt"), "linked")
	require.NoError(t, os.Symlink(filepath.Join(targetDir, "linked.txt"), filepath.Join(shareDir, "link.txt")))
	require.NoError(t, os.Symlink(targetDir, filepath.Join(shareDir, "linked-dir")))
	entries := mustBuildEntries(t, ConfigEntry{Path: shareDir, Name: "files"})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	resp := getResponse(t, http.MethodGet, server.URL+"/zip/0/")
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Disposition"), "files.zip")

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	files := zipContents(t, reader)
	require.Equal(t, "a", files["a.txt"])
	require.Equal(t, "b", files["sub/b.txt"])
	require.Equal(t, "linked", files["link.txt"])
	require.Equal(t, "linked", files["linked-dir/linked.txt"])
}

func TestServerLogsClientIP(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	writeTestFile(t, file, "hello")
	entries := mustBuildEntries(t, ConfigEntry{Path: file})
	var log bytes.Buffer
	server := NewServer(entries, &log)
	req := httptest.NewRequest(http.MethodGet, "/raw/0/", nil)
	req.RemoteAddr = "192.0.2.10:54321"
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	line := log.String()
	require.Contains(t, line, "client_ip=192.0.2.10")
	require.Contains(t, line, "GET /raw/0/ 200")
}

func TestServerRejectsUnsupportedMethodsAndUnknownEntries(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	writeTestFile(t, file, "hello")
	entries := mustBuildEntries(t, ConfigEntry{Path: file})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	resp := getResponse(t, http.MethodPost, server.URL+"/")
	defer resp.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	resp = getResponse(t, http.MethodGet, server.URL+"/raw/9/")
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServerHeadDoesNotWriteBody(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	writeTestFile(t, file, "hello")
	entries := mustBuildEntries(t, ConfigEntry{Path: file})
	server := httptest.NewServer(NewServer(entries, io.Discard))
	defer server.Close()

	resp := getResponse(t, http.MethodHead, server.URL+"/raw/0/")
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, content)
}

func mustBuildEntries(t *testing.T, items ...ConfigEntry) []ShareEntry {
	t.Helper()

	entries, err := BuildEntries(items)
	require.NoError(t, err)
	return entries
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func getBody(t *testing.T, url string) string {
	t.Helper()

	resp := getResponse(t, http.MethodGet, url)
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return string(content)
}

func getResponse(t *testing.T, method string, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func zipContents(t *testing.T, reader *zip.Reader) map[string]string {
	t.Helper()

	files := make(map[string]string)
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "/") {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		files[file.Name] = string(content)
	}
	return files
}
