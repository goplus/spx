package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

type runtimeResolutionFixture struct {
	config   runtimeResolutionConfig
	lock     release.RuntimeLock
	manifest release.RuntimeManifest
}

type roundTripFunc func(*http.Request) (*http.Response, error)

type resolutionErrorReader struct{ err error }

func TestResolveRuntimeReadyDownloadsOnlyManifest(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	fixture.manifest.Provenance.SPXCommit = strings.Repeat("0", 40)
	server, requests := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	got, err := resolveRuntime(context.Background(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateReady || got.Tag != fixture.lock.RuntimeReleaseTag() || !strings.Contains(got.Reason, "current runtime inputs at HEAD") {
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

func TestResolveRuntimeReportsIncompatibleLockIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*release.RuntimeManifest)
		want   string
	}{
		{name: "runtime ABI", mutate: func(manifest *release.RuntimeManifest) {
			manifest.RuntimeABI++
		}, want: "identity does not match lock"},
		{name: "release repository", mutate: func(manifest *release.RuntimeManifest) {
			manifest.ReleaseRepository = "example/runtime"
		}, want: "manifest repository"},
		{name: "lock digest", mutate: func(manifest *release.RuntimeManifest) {
			manifest.LockSHA256 = strings.Repeat("0", 64)
		}, want: "manifest lock SHA-256"},
		{name: "Godot commit", mutate: func(manifest *release.RuntimeManifest) {
			manifest.Provenance.GodotCommit = strings.Repeat("0", 40)
		}, want: "manifest Godot commit"},
		{name: "toolchain", mutate: func(manifest *release.RuntimeManifest) {
			manifest.Provenance.Toolchain.Go = "9.9.9"
		}, want: "manifest toolchain"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRuntimeResolutionFixture(t)
			test.mutate(&fixture.manifest)
			server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
			defer server.Close()
			fixture.config.GitHubAPIURL = server.URL
			fixture.config.HTTPClient = server.Client()

			got, err := resolveRuntime(context.Background(), fixture.config)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != runtimeStateIncompatible || !strings.Contains(got.Reason, test.want) || !strings.Contains(got.Reason, "bump runtime_version") {
				t.Fatalf("resolveRuntime = %#v, want incompatible %q with bump reminder", got, test.want)
			}
		})
	}
}

func TestResolveRuntimeReportsIncompatibleCurrentSourceInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*release.RuntimeManifest)
		want   string
	}{
		{name: "module tree", mutate: func(manifest *release.RuntimeManifest) {
			manifest.Provenance.ModuleTree = strings.Repeat("0", 40)
		}, want: "module tree"},
		{name: "runtime pack source", mutate: func(manifest *release.RuntimeManifest) {
			manifest.Provenance.RuntimePackSourceSHA256 = strings.Repeat("0", 64)
		}, want: "runtime pack source digest"},
		{name: "runtime build recipe", mutate: func(manifest *release.RuntimeManifest) {
			manifest.Provenance.BuildRecipeSHA256 = strings.Repeat("0", 64)
		}, want: "runtime build recipe digest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRuntimeResolutionFixture(t)
			test.mutate(&fixture.manifest)
			server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
			defer server.Close()
			fixture.config.GitHubAPIURL = server.URL
			fixture.config.HTTPClient = server.Client()

			got, err := resolveRuntime(context.Background(), fixture.config)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != runtimeStateIncompatible || !strings.Contains(got.Reason, test.want) || !strings.Contains(got.Reason, "bump runtime_version") {
				t.Fatalf("resolveRuntime = %#v, want incompatible %q with bump reminder", got, test.want)
			}
		})
	}
}

func TestResolveRuntimeFailsClosedWhenCurrentRevisionCannotBeInspected(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	fixture.config.Revision = "refs/heads/does-not-exist"
	server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
	defer server.Close()
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = server.Client()

	_, err := resolveRuntime(context.Background(), fixture.config)
	if err == nil || !strings.Contains(err.Error(), "resolve current SPX commit") {
		t.Fatalf("resolveRuntime error = %v, want revision inspection failure", err)
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
		{name: "incompatible", state: runtimeStateIncompatible, reason: "runtime-v1.2.3 cannot be reused; bump runtime_version"},
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

func TestRunRuntimeResolutionCLI(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	fixture := newRuntimeResolutionFixture(t)
	server, _ := fixture.server(t, http.StatusOK, true, fixture.manifest, nil)
	defer server.Close()
	dir := t.TempDir()
	invalidLock := filepath.Join(dir, "invalid-lock")
	if err := os.WriteFile(invalidLock, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--unknown"}, "flag provided but not defined"},
		{"unexpected argument", []string{"extra"}, "unexpected positional arguments"},
		{"missing lock", []string{"--lock", filepath.Join(dir, "missing")}, "read runtime lock"},
		{"invalid lock", []string{"--lock", invalidLock}, "decode runtime lock"},
		{"invalid API URL", []string{"--lock", fixture.config.LockPath, "--github-api-url", "://invalid"}, "invalid GitHub API URL"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runRuntimeResolution(test.args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runRuntimeResolution = %v, want %q", err, test.want)
			}
		})
	}
	args := []string{"--lock", fixture.config.LockPath, "--github-api-url", server.URL}
	outputPath := filepath.Join(dir, "github-output")
	if err := runRuntimeResolution(append(append([]string(nil), args...), "--github-output", outputPath)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil || !strings.Contains(string(data), "runtime_state=missing\n") || !strings.Contains(string(data), "still a draft") {
		t.Fatalf("GitHub output = %q, error = %v", data, err)
	}
	output, err := os.CreateTemp(dir, "stdout-")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	originalArgs, originalStdout := os.Args, os.Stdout
	t.Cleanup(func() { os.Args, os.Stdout = originalArgs, originalStdout })
	os.Args, os.Stdout = append([]string{"runtime-resolution"}, args...), output
	main()
	data, err = os.ReadFile(output.Name())
	var got runtimeResolution
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil || got.State != runtimeStateMissing || got.Tag != fixture.lock.RuntimeReleaseTag() {
		t.Fatalf("main JSON = %q, error = %v", data, err)
	}
}

func TestFetchGitHubHeadersAndFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "test/accept" || r.Header.Get("User-Agent") != "goplus-spx-runtime-resolution" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request: %s headers %v", r.Method, r.Header)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "response")
	}))
	defer server.Close()
	status, body, err := fetchGitHub(context.Background(), runtimeResolutionConfig{GitHubToken: " secret "}, server.URL, "test/accept")
	if err != nil || status != http.StatusAccepted || string(body) != "response" {
		t.Fatalf("fetchGitHub = %d, %q, %v", status, body, err)
	}
	failure := errors.New("read failed")
	for _, test := range []struct {
		name   string
		url    string
		client *http.Client
		want   string
	}{
		{"invalid URL", "://invalid", nil, "missing protocol scheme"},
		{"transport failure", server.URL, roundTripClient(func(*http.Request) (*http.Response, error) { return nil, failure }), "read failed"},
		{"read failure", server.URL, roundTripClient(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: resolutionErrorReader{failure}}, nil
		}), "read failed"},
		{"oversized body", server.URL, roundTripClient(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxGitHubBodySize+1)))}, nil
		}), "response exceeds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := fetchGitHub(context.Background(), runtimeResolutionConfig{HTTPClient: test.client}, test.url, "test/accept"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("fetchGitHub error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveRuntimeRejectsMalformedReleaseResponses(t *testing.T) {
	fixture := newRuntimeResolutionFixture(t)
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"malformed JSON", "{", "decode GitHub response"},
		{"wrong tag", `{"tag_name":"runtime-v9.9.9"}`, "GitHub returned tag"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := fixture.config
			config.GitHubAPIURL = "https://api.example.test"
			config.HTTPClient = roundTripClient(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			if _, err := resolveRuntime(context.Background(), config); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveRuntime error = %v, want %q", err, test.want)
			}
		})
	}
	fixture.config.Revision = " \t"
	if _, err := currentRuntimeIncompatibility(fixture.config, fixture.lock, fixture.manifest); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("empty revision error = %v", err)
	}
}

func TestVerifyPublishedAssetNamesRejectsAmbiguity(t *testing.T) {
	lock := release.DefaultRuntimeLock()
	var assets []gitHubAsset
	for _, name := range append(append([]string(nil), lock.RequiredAssets...), lock.Manifest, release.SHA256SumsFileName) {
		assets = append(assets, gitHubAsset{Name: name})
	}
	if _, err := verifyPublishedAssetNames(lock, assets); err == nil || !strings.Contains(err.Error(), "no download URL") {
		t.Fatalf("missing manifest URL error = %v", err)
	}
	if _, err := verifyPublishedAssetNames(lock, append(assets, assets[0])); err == nil || !strings.Contains(err.Error(), "duplicate asset") {
		t.Fatalf("duplicate asset error = %v", err)
	}
}

func TestWriteRuntimeResolutionOutputsRejectsInvalidValues(t *testing.T) {
	for _, result := range []runtimeResolution{
		{State: "unknown", Reason: "reason"},
		{State: runtimeStateReady},
		{State: runtimeStateReady, Reason: "reason\nname=value"},
		{State: runtimeStateReady, Reason: "reason\rname=value"},
	} {
		path := filepath.Join(t.TempDir(), "output")
		if err := writeRuntimeResolutionOutputs(path, result); err == nil {
			t.Fatalf("accepted %#v", result)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("created output for invalid result: %v", err)
		}
	}
	if err := writeRuntimeResolutionOutputs(t.TempDir(), runtimeResolution{State: runtimeStateReady, Reason: "verified"}); err == nil || !strings.Contains(err.Error(), "open GitHub output") {
		t.Fatalf("open failure = %v", err)
	}
	for _, base := range []string{"invalid", "https:///path", "://invalid"} {
		if _, err := runtimeReleaseAPIURL(base, "owner/repo", "tag"); err == nil {
			t.Fatalf("accepted API base %q", base)
		}
		if err := verifyReleaseRepository(context.Background(), runtimeResolutionConfig{GitHubAPIURL: base}, "owner/repo"); err == nil {
			t.Fatalf("verified repository with API base %q", base)
		}
	}
}

func (f runtimeResolutionFixture) server(t *testing.T, releaseStatus int, draft bool, manifest release.RuntimeManifest, mutateAssets func(*[]gitHubAsset)) (*httptest.Server, *atomic.Int32) {
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
		if request.URL.Path == "/repos/"+f.lock.ReleaseRepository {
			_ = json.NewEncoder(response).Encode(map[string]string{"full_name": f.lock.ReleaseRepository})
			return
		}
		if releaseStatus != http.StatusOK {
			response.WriteHeader(releaseStatus)
			return
		}

		assetNames := append([]string(nil), f.lock.RequiredAssets...)
		assetNames = append(assetNames, f.lock.Manifest, release.SHA256SumsFileName)
		assets := make([]gitHubAsset, 0, len(assetNames))
		for _, name := range assetNames {
			downloadURL := server.URL + "/unused/" + name
			if name == f.lock.Manifest {
				downloadURL = server.URL + "/manifest"
			}
			assets = append(assets, gitHubAsset{Name: name, BrowserDownloadURL: downloadURL})
		}
		if mutateAssets != nil {
			mutateAssets(&assets)
		}
		_ = json.NewEncoder(response).Encode(map[string]any{
			"tag_name": f.lock.RuntimeReleaseTag(),
			"draft":    draft,
			"assets":   assets,
		})
	}))
	return server, requests
}

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func (r resolutionErrorReader) Read([]byte) (int, error) { return 0, r.err }

func (resolutionErrorReader) Close() error { return nil }

func newRuntimeResolutionFixture(t *testing.T) runtimeResolutionFixture {
	t.Helper()
	repoRoot := gitOutput(t, ".", "rev-parse", "--show-toplevel")
	currentLockPath := filepath.Join(repoRoot, "internal", "release", "runtime.lock.json")
	lock, err := loadRuntimeResolutionLock(currentLockPath)
	if err != nil {
		t.Fatal(err)
	}
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

func gitOutput(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", repoRoot}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(commandArgs, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func roundTripClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}
