package engine

import "testing"

func TestProjectConfigPath(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{
			name:   "desktop prefix",
			prefix: defaultAssetPathPrefix,
			want:   "../.config",
		},
		{
			name:   "web prefix",
			prefix: "",
			want:   ".config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectConfigPath(tt.prefix); got != tt.want {
				t.Fatalf("projectConfigPath(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestBuildFilesystemAssetPath(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	assetPaths.root = "../assets/"

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "keep asset path inside root",
			path: "image.png",
			want: "../assets/image.png",
		},
		{
			name: "clean nested path inside root",
			path: "sprites/../image.png",
			want: "../assets/image.png",
		},
		{
			name: "allow shared external resource",
			path: "../../res/image.png",
			want: "../../res/image.png",
		},
		{
			name: "reject parent traversal",
			path: "../../../../etc/passwd",
			want: "",
		},
		{
			name: "reject sibling directory with same prefix",
			path: "../assets_backup/image.png",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFilesystemAssetPath(tt.path); got != tt.want {
				t.Fatalf("buildFilesystemAssetPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRewriteExtAssetPath(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	assetPaths.prefix = defaultAssetPathPrefix
	assetPaths.extAssetDir = "custom_asset"

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "rewrite root extasset path",
			path: "../custom_asset/image.png",
			want: "../extasset/image.png",
		},
		{
			name: "rewrite nested parent traversal",
			path: "../../custom_asset/image.png",
			want: "../extasset/image.png",
		},
		{
			name: "skip extasset path without parent traversal",
			path: "custom_asset/image.png",
			want: "",
		},
		{
			name: "skip normal asset path",
			path: "../assets/image.png",
			want: "",
		},
		{
			name: "skip substring match",
			path: "../custom_asset_backup/image.png",
			want: "",
		},
		{
			name: "skip nested extasset directory",
			path: "../subdir/custom_asset/image.png",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteExtAssetPath(tt.path); got != tt.want {
				t.Fatalf("rewriteExtAssetPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRewriteExtAssetPathWithoutExtAssetDir(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	assetPaths.prefix = defaultAssetPathPrefix
	assetPaths.extAssetDir = ""

	if got := rewriteExtAssetPath("../anything/image.png"); got != "" {
		t.Fatalf("rewriteExtAssetPath with empty extAssetDir = %q, want empty string", got)
	}
}
