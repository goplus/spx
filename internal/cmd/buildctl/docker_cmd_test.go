package main

import "testing"

func TestParseDockerBuildImagesArgsFromFlag(t *testing.T) {
	cfg, err := parseDockerBuildImagesArgs([]string{"--proxy-url", "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("parseDockerBuildImagesArgs returned error: %v", err)
	}
	if cfg.proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("proxyURL = %q, want %q", cfg.proxyURL, "http://127.0.0.1:7890")
	}
}

func TestParseDockerBuildImagesArgsFromPositional(t *testing.T) {
	cfg, err := parseDockerBuildImagesArgs([]string{"http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("parseDockerBuildImagesArgs returned error: %v", err)
	}
	if cfg.proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("proxyURL = %q, want %q", cfg.proxyURL, "http://127.0.0.1:7890")
	}
}

func TestParseDockerBuildEngineArgsDefault(t *testing.T) {
	cfg, err := parseDockerBuildEngineArgs(nil)
	if err != nil {
		t.Fatalf("parseDockerBuildEngineArgs returned error: %v", err)
	}
	if cfg.godotSrc != "" {
		t.Fatalf("godotSrc = %q, want empty", cfg.godotSrc)
	}
}
