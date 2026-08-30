package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestResolveRuntimeReadyDownloadsOnlyManifest(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, requests := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateReady || got.Tag != fixture.lock.RuntimeReleaseTag() {
		t.Fatalf("resolveRuntime = %#v, want ready %s", got, fixture.lock.RuntimeReleaseTag())
	}
	if gotRequests := requests.Load(); gotRequests != 2 {
		t.Fatalf("HTTP requests = %d, want metadata + manifest only", gotRequests)
	}
}

func TestResolveRuntimeMissing(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, requests := fixture.server(t, http.StatusNotFound, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateMissing || !strings.Contains(got.Reason, "not published") {
		t.Fatalf("resolveRuntime = %#v, want missing", got)
	}
	if gotRequests := requests.Load(); gotRequests != 2 {
		t.Fatalf("HTTP requests = %d, want release lookup and repository confirmation", gotRequests)
	}
}

func TestResolveRuntimeFailsClosedWhenRepositoryCannotBeConfirmed(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusNotFound, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = roundTripClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/repos/"+fixture.lock.ReleaseRepository {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    request,
			}, nil
		}
		return server.Client().Transport.RoundTrip(request)
	})

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "verify release repository") || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("resolveRuntime error = %v, want inaccessible-repository failure", err)
	}
}

func TestResolveRuntimeDraftIsMissing(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusOK, true, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateMissing || !strings.Contains(got.Reason, "draft") {
		t.Fatalf("resolveRuntime = %#v, want draft missing", got)
	}
}

func TestResolveRuntimeAcceptsSameVersionBuildMetadata(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	published := fixture.manifest
	published.RuntimeABI++
	published.ReleaseRepository = "example/runtime"
	published.LockSHA256 = strings.Repeat("0", 64)
	published.Provenance.ModuleTree = strings.Repeat("1", 40)
	published.Provenance.BuildRecipeSHA256 = strings.Repeat("2", 64)
	server, _ := fixture.server(t, http.StatusOK, false, published, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateReady {
		t.Fatalf("resolveRuntime = %#v, want ready", got)
	}
}

func TestResolveRuntimeRejectsManifestVersionMismatch(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	conflicting := fixture.manifest
	conflicting.RuntimeVersion = "9.9.9"
	server, _ := fixture.server(t, http.StatusOK, false, conflicting, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "is not reusable") || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("resolveRuntime error = %v, want version conflict", err)
	}
}

func TestResolveRuntimeRejectsIncompletePublishedAssetSet(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, func(assets *[]gitHubAsset) {
		*assets = (*assets)[1:]
	})
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "is not reusable") || !strings.Contains(err.Error(), "release assets") {
		t.Fatalf("resolveRuntime error = %v, want asset-set conflict", err)
	}
}

func TestResolveRuntimeFailsClosedOnGitHubAPIError(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusServiceUnavailable, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "GitHub API returned Service Unavailable") {
		t.Fatalf("resolveRuntime error = %v, want API failure", err)
	}
}

func TestResolveRuntimeRejectsMissingManifestOnPublicRelease(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = roundTripClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/manifest" {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    request,
			}, nil
		}
		return server.Client().Transport.RoundTrip(request)
	})

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "is not reusable") || !strings.Contains(err.Error(), "download manifest") {
		t.Fatalf("resolveRuntime error = %v, want missing-manifest conflict", err)
	}
}

func TestWriteRuntimeResolutionOutputs(t *testing.T) {
	tests := []struct {
		name   string
		state  string
		reason string
	}{
		{name: "ready", state: runtimeStateReady, reason: "verified runtime-v1.2.3"},
		{name: "missing", state: runtimeStateMissing, reason: "runtime-v1.2.3 is not published"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "github-output")
			resolution := runtimeResolution{State: test.state, Reason: test.reason, Tag: "runtime-v1.2.3"}
			if err := writeRuntimeResolutionOutputs(path, resolution); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range []string{"runtime_state=" + test.state, "runtime_reason=" + test.reason} {
				if !strings.Contains(string(data), line+"\n") {
					t.Fatalf("GitHub output %q does not contain %q", data, line)
				}
			}
		})
	}
}

type runtimeResolutionFixture struct {
	config   runtimeResolutionConfig
	lock     release.RuntimeLock
	manifest release.RuntimeManifest
}

func newRuntimeResolutionFixture(t *testing.T) runtimeResolutionFixture {
	t.Helper()
	lock := release.DefaultRuntimeLock()
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	assetDir := t.TempDir()
	inputs := make([]release.RuntimeAssetInput, 0, len(lock.RequiredAssets))
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(assetDir, name)
		if err := os.WriteFile(path, []byte("asset:"+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, release.RuntimeAssetInput{Name: name, Path: path})
	}
	manifest, err := release.GenerateRuntimeManifest(lock, release.RuntimeProvenance{
		SPXCommit:               strings.Repeat("a", 40),
		GodotCommit:             lock.Godot.Commit,
		ModuleTree:              strings.Repeat("b", 40),
		RuntimePackSourceSHA256: strings.Repeat("c", 64),
		BuildRecipeSHA256:       strings.Repeat("d", 64),
		Toolchain:               lock.Toolchain,
	}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	return runtimeResolutionFixture{
		config: runtimeResolutionConfig{
			LockPath: lockPath,
		},
		lock:     lock,
		manifest: manifest,
	}
}

func (fixture runtimeResolutionFixture) server(t *testing.T, releaseStatus int, draft bool, manifest release.RuntimeManifest, mutateAssets func(*[]gitHubAsset)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	requests := &atomic.Int32{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path == "/manifest" {
			data, err := manifest.JSON()
			if err != nil {
				t.Error(err)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write(data)
			return
		}
		if request.URL.Path == "/repos/"+fixture.lock.ReleaseRepository {
			_ = json.NewEncoder(response).Encode(map[string]string{"full_name": fixture.lock.ReleaseRepository})
			return
		}
		if releaseStatus != http.StatusOK {
			response.WriteHeader(releaseStatus)
			return
		}

		assetNames := append([]string(nil), fixture.lock.RequiredAssets...)
		assetNames = append(assetNames, fixture.lock.Manifest, release.SHA256SumsFileName)
		assets := make([]gitHubAsset, 0, len(assetNames))
		for _, name := range assetNames {
			downloadURL := server.URL + "/unused/" + name
			if name == fixture.lock.Manifest {
				downloadURL = server.URL + "/manifest"
			}
			assets = append(assets, gitHubAsset{Name: name, BrowserDownloadURL: downloadURL})
		}
		if mutateAssets != nil {
			mutateAssets(&assets)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{
			"tag_name": fixture.lock.RuntimeReleaseTag(),
			"draft":    draft,
			"assets":   assets,
		})
	}))
	return server, requests
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func roundTripClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}
