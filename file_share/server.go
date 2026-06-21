package main

import (
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	entries []ShareEntry
	logger  io.Writer
}

type pageData struct {
	Title       string
	Breadcrumbs []breadcrumb
	Rows        []row
	ZipURL      string
}

type breadcrumb struct {
	Name string
	URL  string
}

type row struct {
	Name        string
	Kind        string
	Size        string
	ModTime     string
	BrowseURL   string
	RawURL      string
	DownloadURL string
	ZipURL      string
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; color: #1f2933; }
    a { color: #0b63ce; text-decoration: none; }
    a:hover { text-decoration: underline; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border-bottom: 1px solid #d9e2ec; padding: 8px 10px; text-align: left; }
    th { background: #f0f4f8; font-weight: 600; }
    .crumbs { margin-bottom: 16px; }
    .actions a { margin-right: 12px; white-space: nowrap; }
    .empty { color: #627d98; }
  </style>
</head>
<body>
  <nav class="crumbs">{{range $i, $crumb := .Breadcrumbs}}{{if $i}} / {{end}}<a href="{{$crumb.URL}}">{{$crumb.Name}}</a>{{end}}</nav>
  <h1>{{.Title}}</h1>
  {{if .ZipURL}}<p><a href="{{.ZipURL}}">Download ZIP</a></p>{{end}}
  {{if .Rows}}
  <table>
    <thead>
      <tr><th>Name</th><th>Type</th><th>Size</th><th>Modified</th><th>Actions</th></tr>
    </thead>
    <tbody>
      {{range .Rows}}
      <tr>
        <td>{{if .BrowseURL}}<a href="{{.BrowseURL}}">{{.Name}}</a>{{else}}<a href="{{.RawURL}}">{{.Name}}</a>{{end}}</td>
        <td>{{.Kind}}</td>
        <td>{{.Size}}</td>
        <td>{{.ModTime}}</td>
        <td class="actions">
          {{if .BrowseURL}}<a href="{{.BrowseURL}}">Open</a><a href="{{.ZipURL}}">Zip</a>{{else}}<a href="{{.RawURL}}">Open</a><a href="{{.DownloadURL}}">Download</a>{{end}}
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="empty">Empty directory.</p>
  {{end}}
</body>
</html>
`))

func NewServer(entries []ShareEntry, logger io.Writer) http.Handler {
	if logger == nil {
		logger = io.Discard
	}
	return &Server{entries: entries, logger: logger}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := http.StatusOK
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		fmt.Fprintf(s.logger, "client_ip=%s %s %s %d %s\n", clientIP(r), r.Method, r.URL.Path, status, time.Since(start).Truncate(time.Millisecond))
	}()

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		status = http.StatusMethodNotAllowed
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.route(recorder, r, r.URL.EscapedPath())
	status = recorder.status
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}

func (s *Server) route(w http.ResponseWriter, r *http.Request, requestPath string) {
	switch {
	case requestPath == "/":
		s.renderRoot(w, r)
	case strings.HasPrefix(requestPath, "/browse/"):
		s.handleBrowse(w, r, strings.TrimPrefix(requestPath, "/browse/"))
	case strings.HasPrefix(requestPath, "/raw/"):
		s.handleFile(w, r, strings.TrimPrefix(requestPath, "/raw/"), false)
	case strings.HasPrefix(requestPath, "/download/"):
		s.handleFile(w, r, strings.TrimPrefix(requestPath, "/download/"), true)
	case strings.HasPrefix(requestPath, "/zip/"):
		s.handleZip(w, r, strings.TrimPrefix(requestPath, "/zip/"))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) renderRoot(w http.ResponseWriter, r *http.Request) {
	rows := make([]row, 0, len(s.entries))
	for _, entry := range s.entries {
		info, err := os.Stat(entry.Root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		item := row{
			Name:    entry.Name,
			Kind:    kindName(info),
			Size:    sizeText(info),
			ModTime: modTimeText(info),
		}
		if entry.IsDir {
			item.BrowseURL = fmt.Sprintf("/browse/%d/", entry.ID)
			item.ZipURL = fmt.Sprintf("/zip/%d/", entry.ID)
		} else {
			item.RawURL = fmt.Sprintf("/raw/%d/", entry.ID)
			item.DownloadURL = fmt.Sprintf("/download/%d/", entry.ID)
		}
		rows = append(rows, item)
	}

	s.renderPage(w, pageData{
		Title:       "file_share",
		Breadcrumbs: []breadcrumb{{Name: "Home", URL: "/"}},
		Rows:        rows,
	})
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request, value string) {
	entry, rel, ok := s.lookup(value)
	if !ok || !entry.IsDir {
		http.NotFound(w, r)
		return
	}

	target, ok := resolvePath(entry, rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !info.IsDir() {
		s.servePath(w, r, target, false)
		return
	}

	children, err := os.ReadDir(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]row, 0, len(children))
	for _, child := range children {
		childPath := filepath.Join(target, child.Name())
		childInfo, err := os.Stat(childPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		childRel := joinURLPath(rel, child.Name())
		item := row{
			Name:    child.Name(),
			Kind:    kindName(childInfo),
			Size:    sizeText(childInfo),
			ModTime: modTimeText(childInfo),
		}
		if childInfo.IsDir() {
			item.BrowseURL = entryURL("browse", entry.ID, childRel)
			item.ZipURL = entryURL("zip", entry.ID, childRel)
		} else {
			item.RawURL = entryURL("raw", entry.ID, childRel)
			item.DownloadURL = entryURL("download", entry.ID, childRel)
		}
		rows = append(rows, item)
	}

	s.renderPage(w, pageData{
		Title:       entry.Name,
		Breadcrumbs: makeBreadcrumbs(entry, rel),
		Rows:        rows,
		ZipURL:      entryURL("zip", entry.ID, rel),
	})
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, value string, attachment bool) {
	entry, rel, ok := s.lookup(value)
	if !ok {
		http.NotFound(w, r)
		return
	}

	target, ok := resolvePath(entry, rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	s.servePath(w, r, target, attachment)
}

func (s *Server) handleZip(w http.ResponseWriter, r *http.Request, value string) {
	entry, rel, ok := s.lookup(value)
	if !ok || !entry.IsDir {
		http.NotFound(w, r)
		return
	}

	target, ok := resolvePath(entry, rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		http.NotFound(w, r)
		return
	}

	name := entry.Name
	if rel != "" {
		name = filepath.Base(target)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".zip"))
	if r.Method == http.MethodHead {
		return
	}

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()
	if err := writeZipDir(zipWriter, target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) servePath(w http.ResponseWriter, r *http.Request, target string, attachment bool) {
	file, err := os.Open(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	disposition := "inline"
	if attachment {
		disposition = fmt.Sprintf("attachment; filename=%q", filepath.Base(target))
	}
	w.Header().Set("Content-Disposition", disposition)
	if contentType := mime.TypeByExtension(filepath.Ext(target)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (s *Server) lookup(value string) (ShareEntry, string, bool) {
	parts := strings.SplitN(strings.TrimPrefix(value, "/"), "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return ShareEntry{}, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id < 0 || id >= len(s.entries) {
		return ShareEntry{}, "", false
	}

	rel := ""
	if len(parts) == 2 {
		var err error
		rel, err = url.PathUnescape(parts[1])
		if err != nil {
			return ShareEntry{}, "", false
		}
	}
	return s.entries[id], rel, true
}

func (s *Server) renderPage(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func resolvePath(entry ShareEntry, rel string) (string, bool) {
	if hasDotPathSegment(rel) {
		return "", false
	}
	if !entry.IsDir {
		if rel == "" || rel == "/" {
			return entry.Root, true
		}
		return "", false
	}

	cleanRel := path.Clean("/" + rel)
	if cleanRel == "/" {
		return entry.Root, true
	}
	cleanRel = strings.TrimPrefix(cleanRel, "/")
	return filepath.Join(entry.Root, filepath.FromSlash(cleanRel)), true
}

func hasDotPathSegment(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

func writeZipDir(zipWriter *zip.Writer, root string) error {
	return writeZipDirAt(zipWriter, root, "")
}

func writeZipDirAt(zipWriter *zip.Writer, dir string, prefix string) error {
	children, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, child := range children {
		childPath := filepath.Join(dir, child.Name())
		zipName := path.Join(prefix, child.Name())
		info, err := os.Stat(childPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := writeZipDirAt(zipWriter, childPath, zipName); err != nil {
				return err
			}
			continue
		}
		if err := writeZipFile(zipWriter, childPath, zipName, info); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(zipWriter *zip.Writer, filePath string, zipName string, info os.FileInfo) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(zipName)
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func entryURL(action string, id int, rel string) string {
	if rel == "" {
		return fmt.Sprintf("/%s/%d/", action, id)
	}
	parts := strings.Split(path.Clean("/" + rel)[1:], "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return fmt.Sprintf("/%s/%d/%s", action, id, strings.Join(parts, "/"))
}

func makeBreadcrumbs(entry ShareEntry, rel string) []breadcrumb {
	crumbs := []breadcrumb{{Name: "Home", URL: "/"}, {Name: entry.Name, URL: fmt.Sprintf("/browse/%d/", entry.ID)}}
	if rel == "" {
		return crumbs
	}

	current := ""
	for _, part := range strings.Split(path.Clean(rel), "/") {
		if part == "" || part == "." {
			continue
		}
		current = joinURLPath(current, part)
		crumbs = append(crumbs, breadcrumb{
			Name: part,
			URL:  entryURL("browse", entry.ID, current),
		})
	}
	return crumbs
}

func joinURLPath(base string, name string) string {
	if base == "" || base == "." {
		return path.Clean("/" + name)[1:]
	}
	return path.Clean("/" + base + "/" + name)[1:]
}

func kindName(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	return "file"
}

func sizeText(info os.FileInfo) string {
	if info.IsDir() {
		return "-"
	}
	return fmt.Sprintf("%d B", info.Size())
}

func modTimeText(info os.FileInfo) string {
	return info.ModTime().Format("2006-01-02 15:04:05")
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
