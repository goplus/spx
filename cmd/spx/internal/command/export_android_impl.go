package command

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

type androidBuildConfig struct {
	name       string
	goArch     string
	outputFile string
	ccPrefix   string
}

type androidBuildContext struct {
	libDir       string
	goDir        string
	ndkToolchain string
	minSDK       string
}

// ExportApk exports the current project as an Android APK.
func (cmd *CmdTool) ExportApk() error {
	if err := cmd.prepareExport(); err != nil {
		return err
	}
	cmd.BuildDll()
	if err := cmd.buildAndroidLibraries(); err != nil {
		return fmt.Errorf("failed to build Android libraries: %w", err)
	}

	apkPath := filepath.Join(cmd.ProjectDir, ".builds", "android", "game.apk")
	buildDir := filepath.Dir(apkPath)

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	if _, err := os.Stat(cmd.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", cmd.CmdPath)
	}

	projectFilePath := filepath.Join(cmd.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}

	fmt.Println("importing project resources...")
	execCmd := exec.Command(cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--editor", "--quit")
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	if err := execCmd.Run(); err != nil {
		fmt.Printf("warning: project import failed: %v\n", err)
	}

	fmt.Println("exporting Godot project to APK...")
	execCmd = exec.Command(cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--export-debug", "Android", apkPath)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("APK export failed: %w", err)
	}

	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		return fmt.Errorf("APK export failed: file not created at %s", apkPath)
	} else if err != nil {
		return fmt.Errorf("failed to verify APK output: %w", err)
	}
	log.Println("exported APK successfully:", apkPath)

	if !*cmd.Args.Install {
		return nil
	}

	if _, err := exec.LookPath("adb"); err != nil {
		return fmt.Errorf("adb command not found; ensure Android SDK platform tools are installed and in PATH: %w", err)
	}

	output, err := util.OutputCommand(util.CommandOptions{}, "adb", "devices")
	if err != nil {
		return fmt.Errorf("failed to check connected devices: %w", err)
	}

	if !strings.Contains(string(output), "device\n") {
		return fmt.Errorf("no Android device connected; connect a device and enable USB debugging")
	}
	fmt.Println("installing APK...")
	if err := util.ExecCommand(util.CommandOptions{}, "adb", "install", "-r", apkPath); err != nil {
		return fmt.Errorf("APK installation failed: %w", err)
	}
	fmt.Println("installed APK successfully")
	return nil
}

func (cmd *CmdTool) buildAndroidLibraries() error {
	androidNdkRoot := os.Getenv("ANDROID_NDK_ROOT")
	if androidNdkRoot == "" {
		return fmt.Errorf("ANDROID_NDK_ROOT environment variable is not set")
	}

	paths, err := cmd.resolveAndroidBuildContext(androidNdkRoot)
	if err != nil {
		return err
	}
	if err := prepareAndroidLibraryDir(paths.libDir); err != nil {
		return err
	}
	if err := cmd.buildAndroidSharedLibraries(paths); err != nil {
		return err
	}

	fmt.Println("built Android shared libraries successfully")
	return nil
}

func (cmd *CmdTool) resolveAndroidBuildContext(androidNdkRoot string) (androidBuildContext, error) {
	hostTag, err := resolveAndroidHostTag()
	if err != nil {
		return androidBuildContext{}, err
	}

	return androidBuildContext{
		libDir:       filepath.Join(cmd.ProjectDir, "lib"),
		goDir:        filepath.Join(cmd.ProjectDir, "go"),
		ndkToolchain: filepath.Join(androidNdkRoot, "toolchains", "llvm", "prebuilt", hostTag, "bin"),
		minSDK:       "21",
	}, nil
}

func resolveAndroidHostTag() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "windows-x86_64", nil
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux-x86_64", nil
		case "arm64":
			return "linux-aarch64", nil
		default:
			return "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		return "darwin-x86_64", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func prepareAndroidLibraryDir(libDir string) error {
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}
	return nil
}

func (cmd *CmdTool) buildAndroidSharedLibraries(paths androidBuildContext) error {
	for _, build := range androidBuildConfigs() {
		fmt.Printf("building for %s: %s\n", build.name, paths.goDir)
		if err := cmd.buildAndroidSharedLibrary(paths, build); err != nil {
			return err
		}
	}
	return nil
}

func androidBuildConfigs() []androidBuildConfig {
	return []androidBuildConfig{
		{
			name:       "arm64-v8a",
			goArch:     "arm64",
			outputFile: "libgdspx-android-arm64.so",
			ccPrefix:   "aarch64-linux-android",
		},
		{
			name:       "armeabi-v7a",
			goArch:     "arm",
			outputFile: "libgdspx-android-arm32.so",
			ccPrefix:   "armv7a-linux-androideabi",
		},
	}
}

func (cmd *CmdTool) buildAndroidSharedLibrary(paths androidBuildContext, build androidBuildConfig) error {
	if err := util.ExecCommand(
		util.CommandOptions{
			Dir: paths.goDir,
			Env: []string{
				"CGO_ENABLED=1",
				"GOOS=android",
				"GOARCH=" + build.goArch,
				"CC=" + filepath.Join(paths.ndkToolchain, build.ccPrefix+paths.minSDK+"-clang"),
			},
		},
		"go", "build", "-tags=android,packmode", "-buildmode=c-shared",
		"-o", filepath.Join(paths.libDir, build.outputFile), ".",
	); err != nil {
		return fmt.Errorf("failed to build for %s: %w", build.name, err)
	}
	return nil
}
