package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/release"
)

func TestResolveNewRuntimeWithVerifiedEngine(t *testing.T) {
	fixture := newEngineReuseFixture(t)
	// Pack and recipe changes require a new pack, but not a new engine.
	fixture.manifest.Provenance.RuntimePackSourceSHA256 = strings.Repeat("0", 64)
	fixture.manifest.Provenance.BuildRecipeSHA256 = strings.Repeat("0", 64)
	serveEngineReuse(t, &fixture)
	got, err := resolveRuntime(t.Context(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != runtimeStateMissing || got.EngineState != "reuse" || got.EngineVersion != fixture.lock.RuntimeVersion || got.EngineTag != fixture.lock.RuntimeReleaseTag() || got.EngineManifest != fixture.lock.Manifest {
		t.Fatalf("resolution = %#v", got)
	}
}

func TestResolveEngineReuseRejectsInvalidSource(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*release.RuntimeManifest)
		want   string
	}{
		{"legacy manifest", func(m *release.RuntimeManifest) { m.Provenance.EngineSourceSHA256 = "" }, "no engine identity"},
		{"different engine", func(m *release.RuntimeManifest) { m.Provenance.EngineSourceSHA256 = strings.Repeat("0", 64) }, "engine source digest differs"},
		{"wrong source lock", func(m *release.RuntimeManifest) { m.LockSHA256 = strings.Repeat("0", 64) }, "manifest lock SHA-256"},
		{"wrong source version", func(m *release.RuntimeManifest) { m.RuntimeVersion = "98.0.0" }, "does not match"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newEngineReuseFixture(t)
			tt.mutate(&fixture.manifest)
			serveEngineReuse(t, &fixture)
			got, err := resolveRuntime(t.Context(), fixture.config)
			if err == nil || !strings.Contains(err.Error(), tt.want) || got.EngineState == "reuse" {
				t.Fatalf("resolveRuntime = %#v, %v; want %q without engine reuse", got, err, tt.want)
			}
		})
	}
}

func TestResolveEngineReuseRejectsUnavailablePublishedSource(t *testing.T) {
	failure := errors.New("source transfer interrupted")
	for _, tt := range []struct {
		name, endpoint, body, want string
		status                     int
		err                        error
	}{
		{name: "missing release", endpoint: "release", status: http.StatusNotFound, want: "must be a published release"},
		{name: "draft release", endpoint: "release", body: `{"tag_name":"$TAG","draft":true}`, want: "must be a published release"},
		{name: "wrong release tag", endpoint: "release", body: `{"tag_name":"runtime-v99.1.0"}`, want: "GitHub returned tag"},
		{name: "missing source assets", endpoint: "release", body: `{"tag_name":"$TAG","assets":[]}`, want: "release assets"},
		{name: "malformed metadata", endpoint: "release", body: "{", want: "unexpected end of JSON input"},
		{name: "release transfer", endpoint: "release", err: failure, want: failure.Error()},
		{name: "missing manifest", endpoint: "manifest", status: http.StatusNotFound, want: "engine source download manifest: HTTP 404"},
		{name: "malformed manifest", endpoint: "manifest", body: "{", want: "decode runtime manifest"},
		{name: "manifest transfer", endpoint: "manifest", err: failure, want: failure.Error()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newEngineReuseFixture(t)
			serveEngineReuse(t, &fixture)
			path := "/manifest"
			if tt.endpoint == "release" {
				path = "/repos/" + fixture.lock.ReleaseRepository + "/releases/tags/" + fixture.lock.RuntimeReleaseTag()
			}
			transport := fixture.config.HTTPClient.Transport
			fixture.config.HTTPClient = roundTripClient(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != path {
					return transport.RoundTrip(request)
				}
				if tt.err != nil {
					return nil, tt.err
				}
				status := tt.status
				if status == 0 {
					status = http.StatusOK
				}
				body := strings.ReplaceAll(tt.body, "$TAG", fixture.lock.RuntimeReleaseTag())
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			got, err := resolveRuntime(t.Context(), fixture.config)
			if err == nil || !strings.Contains(err.Error(), tt.want) || got.EngineState == "reuse" {
				t.Fatalf("resolveRuntime = %#v, %v; want %q without engine reuse", got, err, tt.want)
			}
		})
	}
}

func TestResolveEngineReuseRejectsInvalidSelection(t *testing.T) {
	for _, tt := range []struct {
		name, version, repository, want string
	}{
		{name: "unknown version", version: "98.0.0", want: "no runtime lock snapshot"},
		{name: "different repository", repository: "example/runtime", want: "source repository differs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newEngineReuseFixture(t)
			if tt.version != "" {
				fixture.config.ReuseEngineVersion = tt.version
			}
			if tt.repository != "" {
				target := fixture.lock
				target.RuntimeVersion, target.ReleaseRepository = "99.0.0", tt.repository
				writeResolutionLock(t, fixture.config.LockPath, target)
			}
			serveEngineReuse(t, &fixture)
			if _, err := resolveRuntime(t.Context(), fixture.config); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("resolveRuntime error = %v; want %q", err, tt.want)
			}
		})
	}
	fixture := newRuntimeResolutionFixture(t)
	fixture.config.ReuseEngineVersion = fixture.lock.RuntimeVersion
	server, _ := fixture.server(t, http.StatusNotFound, false, fixture.manifest, nil)
	t.Cleanup(server.Close)
	fixture.config.GitHubAPIURL, fixture.config.HTTPClient = server.URL, server.Client()
	if _, err := resolveRuntime(t.Context(), fixture.config); err == nil || !strings.Contains(err.Error(), "different published runtime version") {
		t.Fatalf("same-version source error = %v", err)
	}
}

func TestResolvePublishedTargetTakesPrecedenceOverEngineSelection(t *testing.T) {
	for _, state := range []string{runtimeStateReady, runtimeStateIncompatible} {
		t.Run(state, func(t *testing.T) {
			fixture := newRuntimeResolutionFixture(t)
			fixture.config.ReuseEngineVersion = "unknown-source-must-not-be-resolved"
			if state == runtimeStateIncompatible {
				fixture.manifest.Provenance.RuntimePackSourceSHA256 = strings.Repeat("0", 64)
			}
			server, requests := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
			t.Cleanup(server.Close)
			fixture.config.GitHubAPIURL, fixture.config.HTTPClient = server.URL, server.Client()
			got, err := resolveRuntime(t.Context(), fixture.config)
			if err != nil || got.State != state || got.EngineState != "" || requests.Load() != 2 {
				t.Fatalf("resolveRuntime = %#v, %v with %d requests; want target %s only", got, err, requests.Load(), state)
			}
		})
	}
}

func newEngineReuseFixture(t *testing.T) runtimeResolutionFixture {
	t.Helper()
	fixture := newRuntimeResolutionFixture(t)
	var err error
	fixture.manifest.Provenance.EngineSourceSHA256, err = release.RuntimeEngineSourceSHA256(fixture.config.RepoRoot, fixture.config.Revision, fixture.lock)
	if err != nil {
		t.Fatal(err)
	}
	target := fixture.lock
	target.RuntimeVersion = "99.0.0"
	writeResolutionLock(t, fixture.config.LockPath, target)
	fixture.config.ReuseEngineVersion = fixture.lock.RuntimeVersion
	return fixture
}

func writeResolutionLock(t *testing.T, path string, lock release.RuntimeLock) {
	t.Helper()
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func serveEngineReuse(t *testing.T, fixture *runtimeResolutionFixture) {
	t.Helper()
	target, err := loadRuntimeResolutionLock(fixture.config.LockPath)
	if err != nil {
		t.Fatal(err)
	}
	server, _ := fixture.server(t, http.StatusOK, false, fixture.manifest, nil)
	t.Cleanup(server.Close)
	fixture.config.GitHubAPIURL = server.URL
	fixture.config.HTTPClient = roundTripClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/repos/"+target.ReleaseRepository+"/releases/tags/"+target.RuntimeReleaseTag() {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
		}
		if request.URL.Path == "/repos/"+target.ReleaseRepository {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}
		return server.Client().Transport.RoundTrip(request)
	})
}
