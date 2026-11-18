package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestResolveNewRuntimeWithVerifiedEngine(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	source := fixture.lock
	var err error
	fixture.manifest.Provenance.EngineSourceSHA256, err = release.RuntimeEngineSourceSHA256(fixture.config.RepoRoot, "HEAD", source)
	if err != nil {
		t.Fatal(err)
	}
	// A different resource identity does not prevent engine-only reuse.
	fixture.manifest.Provenance.RuntimePackSourceSHA256 = strings.Repeat("0", 64)
	target := source
	target.RuntimeVersion = "99.0.0"
	data, _ := target.JSON()
	if err := os.WriteFile(fixture.config.LockPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	fixture.config.ReuseEngineVersion = source.RuntimeVersion
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + source.ReleaseRepository:
			w.Write([]byte(`{}`))
		case "/repos/" + source.ReleaseRepository + "/releases/tags/" + target.RuntimeReleaseTag():
			w.WriteHeader(404)
		case "/repos/" + source.ReleaseRepository + "/releases/tags/" + source.RuntimeReleaseTag():
			assets := []gitHubAsset{}
			for _, name := range append(append([]string{}, source.RequiredAssets...), source.Manifest, release.SHA256SumsFileName) {
				assets = append(assets, gitHubAsset{Name: name, BrowserDownloadURL: server.URL + "/manifest"})
			}
			json.NewEncoder(w).Encode(gitHubRelease{TagName: source.RuntimeReleaseTag(), Assets: assets})
		case "/manifest":
			json.NewEncoder(w).Encode(fixture.manifest)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()
	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateMissing || got.EngineState != "reuse" || got.EngineTag != source.RuntimeReleaseTag() {
		t.Fatalf("resolution = %#v", got)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"legacy", func() { fixture.manifest.Provenance.EngineSourceSHA256 = "" }},
		{"different engine", func() { fixture.manifest.Provenance.EngineSourceSHA256 = strings.Repeat("0", 64) }},
		{"wrong source lock", func() { fixture.manifest.LockSHA256 = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			if _, err := resolveRuntime(context.Background(), fixture.config); err == nil {
				t.Fatal("unsafe engine source accepted")
			}
		})
	}
}
