package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestFrontendHandlerServesIndexForRootAndRoutes(t *testing.T) {
	t.Parallel()

	handler, err := newFrontendHandler(fstest.MapFS{
		"index.html":      {Data: []byte("<html>smsdock</html>")},
		"assets/app.js":   {Data: []byte("console.log('smsdock');")},
		"assets/app.css":  {Data: []byte("body{}")},
		"favicon.svg":     {Data: []byte("<svg></svg>")},
		"manifest.webapp": {Data: []byte("{}")},
	})
	if err != nil {
		t.Fatalf("newFrontendHandler returned error: %v", err)
	}

	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootResponse.Code != http.StatusOK {
		t.Fatalf("expected root status 200, got %d", rootResponse.Code)
	}
	if body := rootResponse.Body.String(); body != "<html>smsdock</html>" {
		t.Fatalf("unexpected root body: %q", body)
	}

	routeResponse := httptest.NewRecorder()
	handler.ServeHTTP(routeResponse, httptest.NewRequest(http.MethodGet, "/modems/active", nil))
	if routeResponse.Code != http.StatusOK {
		t.Fatalf("expected route status 200, got %d", routeResponse.Code)
	}
	if body := routeResponse.Body.String(); body != "<html>smsdock</html>" {
		t.Fatalf("unexpected route body: %q", body)
	}
}

func TestFrontendHandlerServesAssetsAnd404ForMissingAsset(t *testing.T) {
	t.Parallel()

	handler, err := newFrontendHandler(fstest.MapFS{
		"index.html":    {Data: []byte("<html>smsdock</html>")},
		"assets/app.js": {Data: []byte("console.log('smsdock');")},
	})
	if err != nil {
		t.Fatalf("newFrontendHandler returned error: %v", err)
	}

	assetResponse := httptest.NewRecorder()
	handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("expected asset status 200, got %d", assetResponse.Code)
	}
	if body := assetResponse.Body.String(); body != "console.log('smsdock');" {
		t.Fatalf("unexpected asset body: %q", body)
	}

	missingAssetResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingAssetResponse, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if missingAssetResponse.Code != http.StatusNotFound {
		t.Fatalf("expected missing asset status 404, got %d", missingAssetResponse.Code)
	}
}

func TestFrontendHandlerRequiresIndex(t *testing.T) {
	t.Parallel()

	_, err := newFrontendHandler(fstest.MapFS{
		"assets/app.js": {Data: []byte("console.log('smsdock');")},
	})
	if err == nil {
		t.Fatal("expected error when index.html is missing")
	}
}

func TestNewFrontendHandlerAllowsDisabledFrontend(t *testing.T) {
	t.Parallel()

	handler, err := NewFrontendHandler("")
	if err != nil {
		t.Fatalf("NewFrontendHandler returned error: %v", err)
	}
	if handler != nil {
		t.Fatal("expected nil handler for empty root")
	}
}

func TestNewFrontendHandlerLoadsDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>smsdock</html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	handler, err := NewFrontendHandler(root)
	if err != nil {
		t.Fatalf("NewFrontendHandler returned error: %v", err)
	}
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
