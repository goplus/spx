package command

import "fmt"

const (
	goosWindows = "windows"
	goosDarwin  = "darwin"
	goosLinux   = "linux"
)

func executableSuffix(goos string) string {
	if goos == goosWindows {
		return ".exe"
	}
	return ""
}

func goBinaryName(goos string) string {
	return "go" + executableSuffix(goos)
}

func sharedLibrarySuffix(goos string) string {
	switch goos {
	case goosWindows:
		return ".dll"
	case goosDarwin:
		return ".dylib"
	default:
		return ".so"
	}
}

func libraryFileName(goos, goarch string) string {
	return fmt.Sprintf("%s-%s-%s%s", ENV_NAME, goos, goarch, sharedLibrarySuffix(goos))
}

func resolveDesktopExportTarget(goos, basePath string) (targetPath, platformName string, err error) {
	switch goos {
	case goosWindows:
		return basePath + executableSuffix(goos), "Win", nil
	case goosDarwin:
		return basePath + ".app", "Mac", nil
	case goosLinux:
		return basePath, "Linux", nil
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", goos)
	}
}
