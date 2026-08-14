package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestResolveRuntimeRejectsPublishedProvenanceConflict(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	conflicting := fixture.manifest
	conflicting.Provenance.ModuleTree = strings.Repeat("0", 40)
	server, _ := fixture.server(t, http.StatusOK, false, conflicting, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "conflicts with the current runtime identity") || !strings.Contains(err.Error(), "module tree") {
		t.Fatalf("resolveRuntime error = %v, want provenance conflict", err)
	}
}

func TestRuntimeBuildRecipeCompatibilityIsExact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		runtimeVersion  string
		manifestDigest  string
		publishedDigest string
		currentDigest   string
		want            bool
	}{
		{
			name:            "future identical recipe",
			runtimeVersion:  "9.9.9",
			manifestDigest:  "unused",
			publishedDigest: "same",
			currentDigest:   "same",
			want:            true,
		},
		{
			name:            "v2.4 migration",
			runtimeVersion:  legacyRuntimeVersion,
			manifestDigest:  legacyRuntimeManifestSHA256,
			publishedDigest: legacyRuntimeRecipeSHA256,
			currentDigest:   semanticRuntimeRecipeSHA256,
			want:            true,
		},
		{
			name:            "v2.4 cannot bypass manifest root",
			runtimeVersion:  legacyRuntimeVersion,
			manifestDigest:  strings.Repeat("0", 64),
			publishedDigest: semanticRuntimeRecipeSHA256,
			currentDigest:   semanticRuntimeRecipeSHA256,
		},
		{
			name:            "different manifest",
			runtimeVersion:  legacyRuntimeVersion,
			manifestDigest:  strings.Repeat("0", 64),
			publishedDigest: legacyRuntimeRecipeSHA256,
			currentDigest:   semanticRuntimeRecipeSHA256,
		},
		{
			name:            "different semantic recipe",
			runtimeVersion:  legacyRuntimeVersion,
			manifestDigest:  legacyRuntimeManifestSHA256,
			publishedDigest: legacyRuntimeRecipeSHA256,
			currentDigest:   strings.Repeat("0", 64),
		},
		{
			name:            "different runtime",
			runtimeVersion:  "2.4.1",
			manifestDigest:  legacyRuntimeManifestSHA256,
			publishedDigest: legacyRuntimeRecipeSHA256,
			currentDigest:   semanticRuntimeRecipeSHA256,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := runtimeBuildRecipeCompatible(
				test.runtimeVersion,
				test.manifestDigest,
				test.publishedDigest,
				test.currentDigest,
			)
			if got != test.want {
				t.Fatalf("runtimeBuildRecipeCompatible() = %t, want %t", got, test.want)
			}
		})
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
	if err == nil || !strings.Contains(err.Error(), "conflicts with the current runtime identity") || !strings.Contains(err.Error(), "release assets") {
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
	if err == nil || !strings.Contains(err.Error(), "conflicts with the current runtime identity") || !strings.Contains(err.Error(), "download manifest") {
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
	repoRoot := gitOutput(t, ".", "rev-parse", "--show-toplevel")
	currentLockPath := filepath.Join(repoRoot, "internal", "release", "runtime.lock.json")
	lock, err := loadRuntimeResolutionLock(currentLockPath)
	if err != nil {
		t.Fatal(err)
	}
	// Resolver fixtures exercise the normal exact-digest path. The published
	// v2.4.0 identity is reserved for the explicit immutable migration tests.
	lock.RuntimeVersion = "9.9.9"
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(t.TempDir(), "runtime.lock.json")
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}
	moduleTree := gitOutput(t, repoRoot, "rev-parse", "--verify", "HEAD:"+lock.Module.Path)
	spxCommit := gitOutput(t, repoRoot, "rev-parse", "--verify", "HEAD^{commit}")
	runtimePackDigest, err := release.RuntimePackSourceSHA256(repoRoot, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	buildRecipeDigest, err := release.RuntimeBuildRecipeSHA256(repoRoot, "HEAD")
	if err != nil {
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
		SPXCommit:               spxCommit,
		GodotCommit:             lock.Godot.Commit,
		ModuleTree:              moduleTree,
		RuntimePackSourceSHA256: runtimePackDigest,
		BuildRecipeSHA256:       buildRecipeDigest,
		Toolchain:               lock.Toolchain,
	}, inputs)
	if err != nil {
		t.Fatal(err)
	}
	return runtimeResolutionFixture{
		config: runtimeResolutionConfig{
			LockPath: lockPath,
			RepoRoot: repoRoot,
			Revision: "HEAD",
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

func gitOutput(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", repoRoot}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func roundTripClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}
