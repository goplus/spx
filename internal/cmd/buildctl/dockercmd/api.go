package dockercmd

type BuildImagesConfig struct {
	ProxyURL string
}

type BuildEngineConfig struct {
	GodotSrc string
}

func Run(args []string) error { return runDocker(args) }
func ParseDockerBuildImagesArgs(args []string) (BuildImagesConfig, error) {
	cfg, err := parseDockerBuildImagesArgs(args)
	return BuildImagesConfig{ProxyURL: cfg.proxyURL}, err
}
func ParseDockerBuildEngineArgs(args []string) (BuildEngineConfig, error) {
	cfg, err := parseDockerBuildEngineArgs(args)
	return BuildEngineConfig{GodotSrc: cfg.godotSrc}, err
}
func RunDockerBuildImages(cfg BuildImagesConfig) error {
	return runDockerBuildImages(dockerBuildImagesConfig{proxyURL: cfg.ProxyURL})
}
func RunDockerBuildEngine(cfg BuildEngineConfig) error {
	return runDockerBuildEngine(dockerBuildEngineConfig{godotSrc: cfg.GodotSrc})
}
func BuildPodmanSConsArgs(godotPath, platform string, sconsArgs []string, useTTY bool) []string {
	return buildPodmanSConsArgs(godotPath, platform, sconsArgs, useTTY)
}
