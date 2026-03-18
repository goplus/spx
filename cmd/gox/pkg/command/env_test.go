package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdToolCheckEnv(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{name: "PreferredSourceExt", filename: preferredMainFile},
		{name: "LegacySourceExt", filename: legacyMainFile},
		{name: "MissingSourceFile", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.filename != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.filename), nil, 0644); err != nil {
					t.Fatalf("write source file: %v", err)
				}
			}

			cmd := &CmdTool{
				TargetDir:  dir,
				FileSuffix: preferredSourceExt,
			}

			err := cmd.CheckEnv()
			if tt.wantErr && err == nil {
				t.Fatal("CheckEnv() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CheckEnv() error = %v, want nil", err)
			}
		})
	}
}
