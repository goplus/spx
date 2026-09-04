package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/release"
)

const (
	runtimeStateReady        = "ready"
	runtimeStateMissing      = "missing"
	runtimeStateIncompatible = "incompatible"
	maxGitHubBodySize        = 8 << 20
)

type runtimeResolutionConfig struct {
	LockPath     string
	RepoRoot     string
	Revision     string
	GitHubAPIURL string
	GitHubToken  string
	HTTPClient   *http.Client
}

type runtimeResolution struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
	Tag    string `json:"tag"`
}

type gitHubRelease struct {
	TagName string        `json:"tag_name"`
	Draft   bool          `json:"draft"`
	Assets  []gitHubAsset `json:"assets"`
}

type gitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func main() {
	if err := runRuntimeResolution(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRuntimeResolution(args []string) error {
	fs := flag.NewFlagSet("runtime_resolution", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	config := runtimeResolutionConfig{}
	var githubOutput string
	fs.StringVar(&config.LockPath, "lock", "internal/release/runtime.lock.json", "runtime lock JSON")
	fs.StringVar(&config.RepoRoot, "repo-root", ".", "SPX repository root")
	fs.StringVar(&config.Revision, "revision", "HEAD", "SPX revision to verify")
	fs.StringVar(&config.GitHubAPIURL, "github-api-url", "https://api.github.com", "GitHub API base URL")
	fs.StringVar(&githubOutput, "github-output", "", "optional GitHub step output file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: runtime_resolution [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	config.GitHubToken = os.Getenv("GH_TOKEN")
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	resolution, err := resolveRuntime(context.Background(), config)
	if err != nil {
		return err
	}
	if githubOutput != "" {
		return writeRuntimeResolutionOutputs(githubOutput, resolution)
	}
	return json.NewEncoder(os.Stdout).Encode(resolution)
}

func resolveRuntime(ctx context.Context, config runtimeResolutionConfig) (runtimeResolution, error) {
	lock, err := loadRuntimeResolutionLock(config.LockPath)
	if err != nil {
		return runtimeResolution{}, err
	}
	tag := lock.RuntimeReleaseTag()
	result := runtimeResolution{Tag: tag}

	apiURL, err := runtimeReleaseAPIURL(config.GitHubAPIURL, lock.ReleaseRepository, tag)
	if err != nil {
		return runtimeResolution{}, err
	}
	status, body, err := fetchGitHub(ctx, config, apiURL, "application/vnd.github+json")
	if err != nil {
		return runtimeResolution{}, fmt.Errorf("resolve runtime release %s: %w", tag, err)
	}
	if status == http.StatusNotFound {
		if err := verifyReleaseRepository(ctx, config, lock.ReleaseRepository); err != nil {
			return runtimeResolution{}, fmt.Errorf("resolve runtime release %s: %w", tag, err)
		}
		result.State = runtimeStateMissing
		result.Reason = fmt.Sprintf("%s is not published", tag)
		return result, nil
	}
	if status != http.StatusOK {
		return runtimeResolution{}, fmt.Errorf("resolve runtime release %s: GitHub API returned %s", tag, http.StatusText(status))
	}

	var published gitHubRelease
	if err := json.Unmarshal(body, &published); err != nil {
		return runtimeResolution{}, fmt.Errorf("resolve runtime release %s: decode GitHub response: %w", tag, err)
	}
	if published.TagName != tag {
		return runtimeResolution{}, runtimeConflict(tag, "GitHub returned tag %q", published.TagName)
	}
	if published.Draft {
		result.State = runtimeStateMissing
		result.Reason = fmt.Sprintf("%s is still a draft", tag)
		return result, nil
	}

	manifestURL, err := verifyPublishedAssetNames(lock, published.Assets)
	if err != nil {
		return runtimeResolution{}, runtimeConflict(tag, "%v", err)
	}
	status, body, err = fetchGitHub(ctx, config, manifestURL, "application/octet-stream")
	if err != nil {
		return runtimeResolution{}, fmt.Errorf("resolve runtime release %s manifest: %w", tag, err)
	}
	if status != http.StatusOK {
		return runtimeResolution{}, runtimeConflict(tag, "download manifest: HTTP %d %s", status, http.StatusText(status))
	}
	manifest, err := release.ParseRuntimeManifestForRelease(body, lock.RuntimeVersion, lock.RequiredAssets)
	if err != nil {
		return runtimeResolution{}, runtimeConflict(tag, "parse manifest: %v", err)
	}
	incompatibility, err := currentRuntimeIncompatibility(config, lock, manifest)
	if err != nil {
		return runtimeResolution{}, fmt.Errorf("resolve current runtime compatibility: %w", err)
	}
	if incompatibility != "" {
		result.State = runtimeStateIncompatible
		result.Reason = fmt.Sprintf("%s cannot be reused by %s: %s; bump runtime_version", tag, strings.TrimSpace(config.Revision), incompatibility)
		return result, nil
	}

	result.State = runtimeStateReady
	result.Reason = fmt.Sprintf("verified published %s against current runtime inputs at %s", tag, strings.TrimSpace(config.Revision))
	return result, nil
}

func loadRuntimeResolutionLock(path string) (release.RuntimeLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return release.RuntimeLock{}, fmt.Errorf("read runtime lock: %w", err)
	}
	lock, err := release.ParseRuntimeLock(data)
	if err != nil {
		return release.RuntimeLock{}, err
	}
	return lock, nil
}

func runtimeReleaseAPIURL(baseURL, repository, tag string) (string, error) {
	repositoryURL, err := runtimeRepositoryAPIURL(baseURL, repository)
	if err != nil {
		return "", err
	}
	return repositoryURL + "/releases/tags/" + url.PathEscape(tag), nil
}

func runtimeRepositoryAPIURL(baseURL, repository string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid GitHub API URL %q", baseURL)
	}
	return parsed.String() + "/repos/" + repository, nil
}

func verifyReleaseRepository(ctx context.Context, config runtimeResolutionConfig, repository string) error {
	repositoryURL, err := runtimeRepositoryAPIURL(config.GitHubAPIURL, repository)
	if err != nil {
		return err
	}
	status, _, err := fetchGitHub(ctx, config, repositoryURL, "application/vnd.github+json")
	if err != nil {
		return fmt.Errorf("verify release repository %s: %w", repository, err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("verify release repository %s: GitHub API returned HTTP %d %s", repository, status, http.StatusText(status))
	}
	return nil
}

func fetchGitHub(ctx context.Context, config runtimeResolutionConfig, requestURL, accept string) (int, []byte, error) {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", "goplus-spx-runtime-resolution")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(config.GitHubToken); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGitHubBodySize+1))
	if err != nil {
		return 0, nil, err
	}
	if len(body) > maxGitHubBodySize {
		return 0, nil, fmt.Errorf("response exceeds %d bytes", maxGitHubBodySize)
	}
	return response.StatusCode, body, nil
}

func verifyPublishedAssetNames(lock release.RuntimeLock, assets []gitHubAsset) (string, error) {
	expected := append([]string(nil), lock.RequiredAssets...)
	expected = append(expected, lock.Manifest, release.SHA256SumsFileName)
	slices.Sort(expected)

	actual := make([]string, 0, len(assets))
	seen := make(map[string]struct{}, len(assets))
	manifestURL := ""
	for _, asset := range assets {
		if _, ok := seen[asset.Name]; ok {
			return "", fmt.Errorf("release contains duplicate asset %q", asset.Name)
		}
		seen[asset.Name] = struct{}{}
		actual = append(actual, asset.Name)
		if asset.Name == lock.Manifest {
			manifestURL = asset.BrowserDownloadURL
		}
	}
	slices.Sort(actual)
	if !slices.Equal(actual, expected) {
		return "", fmt.Errorf("release assets = %v, want %v", actual, expected)
	}
	if strings.TrimSpace(manifestURL) == "" {
		return "", errors.New("release manifest has no download URL")
	}
	return manifestURL, nil
}

func currentRuntimeIncompatibility(config runtimeResolutionConfig, lock release.RuntimeLock, manifest release.RuntimeManifest) (string, error) {
	if err := manifest.ValidateForLock(lock); err != nil {
		return err.Error(), nil
	}

	// SPXCommit is traceability metadata, not a reuse input. SPX-only commits
	// remain reusable when all scoped runtime source identities are unchanged.
	repoRoot := filepath.Clean(config.RepoRoot)
	revision := strings.TrimSpace(config.Revision)
	if revision == "" {
		return "", errors.New("SPX revision must not be empty")
	}
	commit, err := gitRevision(repoRoot, revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve current SPX commit: %w", err)
	}
	moduleTree, err := gitRevision(repoRoot, commit+":"+lock.Module.Path)
	if err != nil {
		return "", fmt.Errorf("resolve current module tree: %w", err)
	}
	if manifest.Provenance.ModuleTree != moduleTree {
		return fmt.Sprintf("module tree = %s, current runtime input = %s", manifest.Provenance.ModuleTree, moduleTree), nil
	}
	runtimePackDigest, err := release.RuntimePackSourceSHA256(repoRoot, commit)
	if err != nil {
		return "", err
	}
	if manifest.Provenance.RuntimePackSourceSHA256 != runtimePackDigest {
		return fmt.Sprintf("runtime pack source digest = %s, current runtime input = %s", manifest.Provenance.RuntimePackSourceSHA256, runtimePackDigest), nil
	}
	buildRecipeDigest, err := release.RuntimeBuildRecipeSHA256(repoRoot, commit)
	if err != nil {
		return "", err
	}
	if manifest.Provenance.BuildRecipeSHA256 != buildRecipeDigest {
		return fmt.Sprintf("runtime build recipe digest = %s, current runtime input = %s", manifest.Provenance.BuildRecipeSHA256, buildRecipeDigest), nil
	}
	return "", nil
}

func gitRevision(repoRoot, revision string) (string, error) {
	command := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "--end-of-options", revision)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if detail := strings.TrimSpace(string(exitErr.Stderr)); detail != "" {
				return "", errors.New(detail)
			}
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func runtimeConflict(tag, format string, args ...any) error {
	return fmt.Errorf("runtime %s is not reusable: %s", tag, fmt.Sprintf(format, args...))
}

func writeRuntimeResolutionOutputs(path string, resolution runtimeResolution) error {
	if resolution.State != runtimeStateReady && resolution.State != runtimeStateMissing && resolution.State != runtimeStateIncompatible {
		return fmt.Errorf("invalid runtime state %q", resolution.State)
	}
	if resolution.Reason == "" || strings.ContainsAny(resolution.Reason, "\r\n") {
		return errors.New("runtime reason must be a non-empty single line")
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub output: %w", err)
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "runtime_state=%s\nruntime_reason=%s\n", resolution.State, resolution.Reason)
	return err
}
