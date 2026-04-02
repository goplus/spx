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

	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	if _, err := os.Stat(cmd.CmdPath); os.IsNotExist(err) {
		return fmt.Errorf("Godot binary not found at %s", cmd.CmdPath)
	}

	projectFilePath := filepath.Join(cmd.ProjectDir, "project.godot")
	if _, err := os.Stat(projectFilePath); os.IsNotExist(err) {
		return fmt.Errorf("Godot project file not found at %s", projectFilePath)
	}

	fmt.Println("Importing project resources...")
	execCmd := exec.Command(cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--editor", "--quit")
	if err := execCmd.Run(); err != nil {
	}

	fmt.Println("Exporting Godot project to APK...")
	execCmd = exec.Command(cmd.CmdPath, "--headless", "--path", cmd.ProjectDir, "--export-debug", "Android", apkPath)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		fmt.Println("APK export failed: %w", err)
	}

	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		fmt.Println("APK export failed: file not created at ", apkPath)
		return nil
	}
	log.Println("APK export completed successfully!", apkPath)

	_, err := exec.LookPath("adb")
	if err != nil {
		fmt.Println("adb command not found. Please ensure Android SDK platform tools are installed and in your PATH")
		return nil
	}

	execCmd = exec.Command("adb", "devices")
	output, err := execCmd.Output()
	if err != nil {
		fmt.Println("failed to check for connected devices:", err)
		return nil
	}

	if !strings.Contains(string(output), "device\n") {
		fmt.Println("no Android device connected. Please connect a device and enable USB debugging")
		return nil
	}

	if *cmd.Args.Install {
		fmt.Println("Installing APK...")
		execCmd = exec.Command("adb", "install", "-r", apkPath)
		if err := execCmd.Run(); err != nil {
			fmt.Println("APK installation failed:", err)
			return nil
		}
		fmt.Println("APK installation successful!")
	}
	return nil
}

func (cmd *CmdTool) buildAndroidLibraries() error {
	androidNdkRoot := os.Getenv("ANDROID_NDK_ROOT")
	if androidNdkRoot == "" {
		fmt.Println("ANDROID_NDK_ROOT environment variable is not set")
		return nil
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

	fmt.Println("Build android so completed successfully!")
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
	if err := os.MkdirAll(libDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}
	return nil
}

func (cmd *CmdTool) buildAndroidSharedLibraries(paths androidBuildContext) error {
	for _, build := range androidBuildConfigs() {
		fmt.Printf("Building for %s... %s\n", build.name, paths.goDir)
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
