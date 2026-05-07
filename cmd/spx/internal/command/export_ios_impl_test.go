package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIosExportProjectOnly(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "true",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/export_project_only=true
`,
			want: true,
		},
		{
			name: "false",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/export_project_only=false
`,
			want: false,
		},
		{
			name:    "quoted and crlf",
			content: "[preset.0]\r\nname=\"iOS\"\r\n\r\n[preset.0.options]\r\napplication/export_project_only=\"true\"\r\n",
			want:    true,
		},
		{
			name: "spaces around assignment",
			content: `
[preset.0]
name = "iOS"

[preset.0.options]
application/export_project_only = true
`,
			want: true,
		},
		{
			name: "missing key",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/icon_interpolation=4
`,
			want: false,
		},
		{
			name: "ignore non-ios preset",
			content: `
[preset.0]
name="Android"

[preset.0.options]
application/export_project_only=true
`,
			want: false,
		},
		{
			name: "use matching ios preset options",
			content: `
[preset.0]
name="Android"

[preset.0.options]
application/export_project_only=true

[preset.1]
name="iOS"

[preset.1.options]
application/export_project_only=false
`,
			want: false,
		},
		{
			name: "ignore matching key outside preset options",
			content: `
[preset.0]
name="iOS"
application/export_project_only=true

[preset.0.options]
application/icon_interpolation=4
`,
			want: false,
		},
		{
			name: "ignore non-preset section",
			content: `
[preset.0]
name="Android"

[other]
application/export_project_only=true

[preset.1]
name="iOS"

[preset.1.options]
application/export_project_only=true
`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseIosExportProjectOnly([]byte(tt.content)); got != tt.want {
				t.Fatalf("parseIosExportProjectOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateIosExportOutputProjectOnly(t *testing.T) {
	outputDir := t.TempDir()
	output := iosExportOutput{
		buildDir:         outputDir,
		ipaPath:          filepath.Join(outputDir, "Game.ipa"),
		xcodeProjectPath: filepath.Join(outputDir, "Game.xcodeproj"),
	}

	if err := os.MkdirAll(output.xcodeProjectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", output.xcodeProjectPath, err)
	}

	if err := validateIosExportOutput(output, true); err != nil {
		t.Fatalf("validateIosExportOutput(projectOnly=true) returned error: %v", err)
	}
}

func TestValidateIosExportOutputRequiresIPAWhenNotProjectOnly(t *testing.T) {
	outputDir := t.TempDir()
	output := iosExportOutput{
		buildDir:         outputDir,
		ipaPath:          filepath.Join(outputDir, "Game.ipa"),
		xcodeProjectPath: filepath.Join(outputDir, "Game.xcodeproj"),
	}

	if err := os.WriteFile(output.ipaPath, []byte("ipa"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", output.ipaPath, err)
	}

	if err := validateIosExportOutput(output, false); err != nil {
		t.Fatalf("validateIosExportOutput(projectOnly=false) returned error: %v", err)
	}
}
