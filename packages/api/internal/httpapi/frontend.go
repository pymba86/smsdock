package httpapi

import (
	"bytes"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type frontendHandler struct {
	fs    fs.FS
	files http.Handler
}

func NewFrontendHandler(root string) (http.Handler, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}

	indexPath := filepath.Join(root, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		return nil, fmt.Errorf("stat frontend index: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("frontend index is a directory: %s", indexPath)
	}

	return newFrontendHandler(os.DirFS(root))
}

func newFrontendHandler(frontendFS fs.FS) (http.Handler, error) {
	info, err := fs.Stat(frontendFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("stat frontend index: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("frontend index is a directory")
	}

	return &frontendHandler{
		fs:    frontendFS,
		files: http.FileServer(http.FS(frontendFS)),
	}, nil
}

func (h *frontendHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.NotFound(writer, request)
		return
	}

	cleanPath := path.Clean("/" + request.URL.Path)
	if cleanPath == "/" {
		h.serveFile(writer, request, "index.html")
		return
	}

	relativePath := strings.TrimPrefix(cleanPath, "/")
	if info, err := fs.Stat(h.fs, relativePath); err == nil && !info.IsDir() {
		h.files.ServeHTTP(writer, request)
		return
	}

	if looksLikeAssetPath(relativePath) {
		http.NotFound(writer, request)
		return
	}

	h.serveFile(writer, request, "index.html")
}

func (h *frontendHandler) serveFile(writer http.ResponseWriter, request *http.Request, path string) {
	content, err := fs.ReadFile(h.fs, strings.TrimPrefix(path, "/"))
	if err != nil {
		http.NotFound(writer, request)
		return
	}

	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}

	http.ServeContent(writer, request, filepath.Base(path), time.Time{}, bytes.NewReader(content))
}

func looksLikeAssetPath(path string) bool {
	return strings.Contains(filepath.Base(path), ".")
}
