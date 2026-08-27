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

func libraryFileName(prefix, goos, goarch string) string {
	return fmt.Sprintf("%s-%s-%s%s", prefix, goos, goarch, sharedLibrarySuffix(goos))
}

func desktopExportPlatformName(goos string) (string, error) {
	switch goos {
	case goosWindows:
		return "Win", nil
	case goosDarwin:
		return "Mac", nil
	case goosLinux:
		return "Linux", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}
}

func resolveDesktopExportTarget(goos, basePath string) (targetPath, platformName string, err error) {
	platformName, err = desktopExportPlatformName(goos)
	if err != nil {
		return "", "", err
	}
	if goos == goosWindows {
		return basePath + ".exe", platformName, nil
	}
	if goos == goosDarwin {
		return basePath + ".app", platformName, nil
	}
	return basePath, platformName, nil
}
