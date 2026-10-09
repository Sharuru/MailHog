package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestStaticETagAndFontCache(t *testing.T) {
	font := []byte("sarasa-face")
	css := []byte("body{}")
	js := []byte("var x;")
	assets := map[string][]byte{
		"assets/css/fonts/sarasa-mono-j-regular.ttf": font,
		"assets/css/customize.css":                   css,
		"assets/js/controllers.js":                   js,
	}
	ui := Web{asset: func(name string) ([]byte, error) {
		body, ok := assets[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return body, nil
	}}

	fontRes := getStatic(t, ui, "assets/css/{{file}}", "fonts/sarasa-mono-j-regular.ttf", "")
	if fontRes.Code != http.StatusOK {
		t.Fatalf("font status = %d", fontRes.Code)
	}
	if fontRes.Body.String() != string(font) {
		t.Fatalf("font body = %q", fontRes.Body.String())
	}
	etag := fontRes.Header().Get("ETag")
	if etag == "" || !strings.HasPrefix(etag, `"`) || strings.Contains(etag, "W/") {
		t.Fatalf("font ETag = %q, want a strong quoted tag", etag)
	}
	if cache := fontRes.Header().Get("Cache-Control"); cache != fontCacheControl || strings.Contains(cache, "immutable") {
		t.Fatalf("font Cache-Control = %q", cache)
	}
	if maxAge(t, fontRes) <= maxAgeOf(t, assetCacheControl) {
		t.Fatalf("font max-age %d is not longer than other assets", maxAge(t, fontRes))
	}

	notModified := getStatic(t, ui, "assets/css/{{file}}", "fonts/sarasa-mono-j-regular.ttf", etag)
	if notModified.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match status = %d", notModified.Code)
	}
	if notModified.Body.Len() != 0 {
		t.Fatalf("304 body = %d bytes, want none", notModified.Body.Len())
	}
	if notModified.Header().Get("ETag") != etag {
		t.Fatalf("304 ETag = %q", notModified.Header().Get("ETag"))
	}

	weak := getStatic(t, ui, "assets/css/{{file}}", "fonts/sarasa-mono-j-regular.ttf", "W/"+etag)
	if weak.Code != http.StatusNotModified || weak.Body.Len() != 0 {
		t.Fatalf("weak validator status = %d, body %d", weak.Code, weak.Body.Len())
	}

	miss := getStatic(t, ui, "assets/css/{{file}}", "fonts/sarasa-mono-j-regular.ttf", `"missing"`)
	if miss.Code != http.StatusOK || miss.Body.String() != string(font) {
		t.Fatalf("mismatched validator status = %d", miss.Code)
	}

	cssRes := getStatic(t, ui, "assets/css/{{file}}", "customize.css", "")
	jsRes := getStatic(t, ui, "assets/js/{{file}}", "controllers.js", "")
	if cssRes.Header().Get("Cache-Control") != assetCacheControl || cssRes.Header().Get("ETag") == "" {
		t.Fatalf("css headers = %v", cssRes.Header())
	}
	if jsRes.Header().Get("Cache-Control") != assetCacheControl || jsRes.Header().Get("ETag") == "" {
		t.Fatalf("js headers = %v", jsRes.Header())
	}
	if cssRes.Header().Get("ETag") == etag {
		t.Fatal("css ETag collided with the font")
	}
}

func getStatic(t *testing.T, ui Web, pattern, file, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	query := req.URL.Query()
	query.Set(":file", file)
	req.URL.RawQuery = query.Encode()
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	res := httptest.NewRecorder()
	ui.Static(pattern)(res, req)
	return res
}

func maxAge(t *testing.T, res *httptest.ResponseRecorder) int {
	t.Helper()
	return maxAgeOf(t, res.Header().Get("Cache-Control"))
}

func maxAgeOf(t *testing.T, header string) int {
	t.Helper()
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			n, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
			if err != nil {
				t.Fatalf("max-age %q: %v", header, err)
			}
			return n
		}
	}
	t.Fatalf("no max-age in %q", header)
	return 0
}
