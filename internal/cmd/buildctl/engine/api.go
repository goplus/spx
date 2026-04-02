package engine

import "net/http"

type DownloadConfig struct {
	Runtime  bool
	Platform string
	Mode     string
}

type DownloadEnv struct {
	RepoRoot    string
	Version     string
	Platform    string
	Arch        string
	GoBinDir    string
	TemplateDir string
	CacheDir    string
	URLPrefix   string
}

type BuildConfig struct {
	Target   string
	Platform string
	Mode     string
}

type ExecConfig struct {
	LockDir string
	Workdir string
	Script  string
	Command []string
}

type BuildShellConfig struct {
	Target   string
	Platform string
	Mode     string
}

type BuildShellPlan = engineBuildShellPlan

func Run(args []string) error { return runEngine(args) }
func ParseEngineDownloadArgs(args []string) (DownloadConfig, error) {
	cfg, err := parseEngineDownloadArgs(args)
	return DownloadConfig{Runtime: cfg.runtime, Platform: cfg.platform, Mode: cfg.mode}, err
}
func ParseEngineBuildArgs(args []string) (BuildConfig, error) {
	cfg, err := parseEngineBuildArgs(args)
	return BuildConfig{Target: cfg.target, Platform: cfg.platform, Mode: cfg.mode}, err
}
func ParseEngineExecArgs(args []string) (ExecConfig, error) {
	cfg, err := parseEngineExecArgs(args)
	return ExecConfig{LockDir: cfg.lockDir, Workdir: cfg.workdir, Script: cfg.script, Command: cfg.command}, err
}
func ParseEnvExportEngineBuildShellArgs(args []string) (BuildShellConfig, error) {
	cfg, err := parseEnvExportEngineBuildShellArgs(args)
	return BuildShellConfig{Target: cfg.target, Platform: cfg.platform, Mode: cfg.mode}, err
}
func ResolveEngineBuildShellPlan(repoRoot string, cfg BuildShellConfig) (BuildShellPlan, error) {
	return resolveEngineBuildShellPlan(repoRoot, envExportEngineBuildShellConfig{target: cfg.Target, platform: cfg.Platform, mode: cfg.Mode})
}
func (plan BuildShellPlan) ShellExports() string { return engineBuildShellPlan(plan).shellExports() }
func DownloadEngineAssets(cfg DownloadConfig, repoRoot string) error {
	return downloadEngineAssets(engineDownloadConfig{runtime: cfg.Runtime, platform: cfg.Platform, mode: cfg.Mode}, repoRoot)
}
func BuildEngine(cfg BuildConfig, repoRoot string) error {
	return buildEngine(engineBuildConfig{target: cfg.Target, platform: cfg.Platform, mode: cfg.Mode}, repoRoot)
}
func ExecEngineCommand(cfg ExecConfig, repoRoot string) error {
	return execEngineCommand(engineExecConfig{lockDir: cfg.LockDir, workdir: cfg.Workdir, script: cfg.Script, command: cfg.Command}, repoRoot)
}
func SConsScript(commands []string) string { return sconsScript(commands) }
func MergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	return mergeStringMaps(base, extra)
}
func PopulateWebTemplateCopies(srcZip, templateDir string) error {
	return populateWebTemplateCopies(srcZip, templateDir)
}
func DetectStaleEngineBuildLock(lockDir string) (bool, string, error) {
	return detectStaleEngineBuildLock(lockDir)
}
func AcquireEngineBuildLock(lockDir string) error            { return acquireEngineBuildLock(lockDir) }
func ExtractZip(srcZip, dstDir string) error                 { return extractZip(srcZip, dstDir) }
func FetchURLToFile(url, dst string) error                   { return fetchURLToFile(url, dst) }
func LinkOrCopyFile(src, dst string) error                   { return linkOrCopyFile(src, dst) }
func ShouldRefreshPreparedAssets() bool                      { return shouldRefreshPreparedAssets() }
func WebModeReleaseTemplateName(mode string) (string, error) { return webModeReleaseTemplateName(mode) }
func WebModeCachedTemplatePath(version, mode string) (string, error) {
	return webModeCachedTemplatePath(version, mode)
}
func PrepareHostEditorAsset(repoRoot string) error {
	env, err := EngineDownloadResolveEnv(repoRoot, "")
	if err != nil {
		return err
	}
	return downloadPlatformAssets(env, "editor", true)
}

func SetEngineDownloadResolveEnv(fn func(repoRoot, platform string) (DownloadEnv, error)) func() {
	old := EngineDownloadResolveEnv
	EngineDownloadResolveEnv = func(repoRoot, platform string) (engineDownloadEnv, error) {
		env, err := fn(repoRoot, platform)
		if err != nil {
			return engineDownloadEnv{}, err
		}
		return engineDownloadEnv{
			repoRoot:    env.RepoRoot,
			version:     env.Version,
			platform:    env.Platform,
			arch:        env.Arch,
			goBinDir:    env.GoBinDir,
			templateDir: env.TemplateDir,
			cacheDir:    env.CacheDir,
			urlPrefix:   env.URLPrefix,
		}, nil
	}
	return func() {
		EngineDownloadResolveEnv = old
	}
}

func SetEngineDownloadHTTPClient(client *http.Client) func() {
	old := EngineDownloadHTTPClient
	EngineDownloadHTTPClient = client
	return func() {
		EngineDownloadHTTPClient = old
	}
}
