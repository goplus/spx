package shared

import "fmt"

func validateSetupMode(mode string) error {
	switch mode {
	case "runtime", "web", "full":
		return nil
	default:
		return fmt.Errorf("unsupported setup-mode: %s", mode)
	}
}

func validateWebMode(mode string) error {
	switch mode {
	case "normal", "worker", "minigame", "miniprogram":
		return nil
	default:
		return fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func validateOptionalPlatform(platform string) error {
	switch platform {
	case "", "android", "ios", "web", "linux", "windows", "macos":
		return nil
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}
