package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseToolSetupSConsArgsDefault(t *testing.T) {
	if _, err := parseToolSetupSConsArgs(nil); err != nil {
		t.Fatalf("parseToolSetupSConsArgs returned error: %v", err)
	}
}

func TestParseToolSetupJDKArgsDefault(t *testing.T) {
	if _, err := parseToolSetupJDKArgs(nil); err != nil {
		t.Fatalf("parseToolSetupJDKArgs returned error: %v", err)
	}
}

func TestParseToolSetupEMSDKArgsDefault(t *testing.T) {
	if _, err := parseToolSetupEMSDKArgs(nil); err != nil {
		t.Fatalf("parseToolSetupEMSDKArgs returned error: %v", err)
	}
}

func TestParseJavaMajorVersion(t *testing.T) {
	cases := []struct {
		output string
		want   int
		ok     bool
	}{
		{output: "openjdk version \"17.0.8\" 2023-07-18", want: 17, ok: true},
		{output: "java version \"1.8.0_402\"", want: 8, ok: true},
		{output: "garbage", want: 0, ok: false},
	}

	for _, tc := range cases {
		got, ok := parseJavaMajorVersion(tc.output)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("parseJavaMajorVersion(%q) = (%d,%v), want (%d,%v)", tc.output, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPrependToPathDedupes(t *testing.T) {
	got := prependToPath("/usr/bin:/bin", "/custom/bin", "/usr/bin")
	want := strings.Join([]string{"/custom/bin", "/usr/bin", "/bin"}, string(filepath.ListSeparator))
	if got != want {
		t.Fatalf("prependToPath = %q, want %q", got, want)
	}
}

func TestSelectEMSDKExports(t *testing.T) {
	before := map[string]string{
		"PATH": "/usr/bin",
		"HOME": "/tmp/home",
	}
	after := map[string]string{
		"PATH":         "/emsdk/bin:/usr/bin",
		"HOME":         "/tmp/home",
		"EMSDK":        "/tmp/emsdk",
		"EM_CONFIG":    "/tmp/.emscripten",
		"UNRELATED":    "value",
		"JAVA_HOME":    "/tmp/jdk",
		"EMSDK_PYTHON": "/tmp/python",
	}
	exports := selectEMSDKExports(before, after)
	if len(exports) != 5 {
		t.Fatalf("unexpected exports: %#v", exports)
	}
	if exports["UNRELATED"] != "" {
		t.Fatalf("unexpected unrelated export: %#v", exports)
	}
}

func TestResolveEMSDKEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))

	env, err := resolveEMSDKEnvironment()
	if err != nil {
		t.Fatalf("resolveEMSDKEnvironment returned error: %v", err)
	}

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(env.rootDir, filepath.Join("Library", "Application Support", "emsdk")) {
			t.Fatalf("unexpected darwin emsdk root: %s", env.rootDir)
		}
	case "linux":
		if !strings.Contains(env.rootDir, filepath.Join(".local", "share", "emsdk")) {
			t.Fatalf("unexpected linux emsdk root: %s", env.rootDir)
		}
	case "windows":
		if !strings.Contains(strings.ToLower(env.rootDir), strings.ToLower(filepath.Join("AppData", "emsdk"))) {
			t.Fatalf("unexpected windows emsdk root: %s", env.rootDir)
		}
	}
}

func TestResolveEMSDKVerificationEnvironmentAddsConfigAndCache(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "emsdk")
	empp := filepath.Join(repoDir, "upstream", "emscripten", emscriptenCPPExecutableName())
	if err := os.MkdirAll(filepath.Dir(empp), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(empp, []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	oldResolve := resolveEMSDKShellExportsFn
	resolveEMSDKShellExportsFn = func() (map[string]string, error) {
		return map[string]string{
			"PATH": filepath.Join(repoDir, "upstream", "emscripten") + string(filepath.ListSeparator) + os.Getenv("PATH"),
		}, nil
	}
	defer func() {
		resolveEMSDKShellExportsFn = oldResolve
	}()

	env, emppPath, err := resolveEMSDKVerificationEnvironment(emsdkEnvironment{
		rootDir: root,
		repoDir: repoDir,
	})
	if err != nil {
		t.Fatalf("resolveEMSDKVerificationEnvironment returned error: %v", err)
	}
	if emppPath != empp {
		t.Fatalf("unexpected em++ path: %s", emppPath)
	}
	if env["EM_CONFIG"] != filepath.Join(repoDir, ".emscripten") {
		t.Fatalf("unexpected EM_CONFIG: %q", env["EM_CONFIG"])
	}
	if env["EM_CACHE"] != filepath.Join(repoDir, "upstream", "emscripten", "cache") {
		t.Fatalf("unexpected EM_CACHE: %q", env["EM_CACHE"])
	}
	if !dirExists(env["EM_CACHE"]) {
		t.Fatalf("expected EM_CACHE directory to exist: %s", env["EM_CACHE"])
	}
}
