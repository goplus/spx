// Package releasemeta provides shared SPX/runtime release metadata.
package releasemeta

const (
	// RuntimeURLBase is the base URL for downloading runtime executables.
	// Format: https://github.com/goplus/godot/releases/download/spx{runtimeVersion}/{platform}-{arch}.zip
	RuntimeURLBase = "https://github.com/goplus/godot/releases/download/"

	// SpxReleaseURLBase is the base URL for downloading SPX release assets.
	// Format: https://github.com/goplus/spx/releases/download/{spxTag}/{assetName}
	SpxReleaseURLBase = "https://github.com/goplus/spx/releases/download/"

	// RuntimeTag is the filename prefix for the runtime binary/pck files.
	RuntimeTag = "gdspxrt"
)

// ReleaseMeta captures the selected SPX/runtime asset mapping.
type ReleaseMeta struct {
	SPXVersion string
	Runtime    RuntimeRelease
	Pck        PckRelease
}

// RuntimeRelease describes the Godot runtime bundle used by SPX.
type RuntimeRelease struct {
	Version string
}

// PckRelease describes the packaged SPX runtime assets published from the spx repo.
type PckRelease struct {
	SPXTag  string
	Version string
}

var defaultPckRelease = PckRelease{
	// Current packaged runtime assets are only published from the pre.30 SPX release.
	SPXTag:  "v2.0.0-pre.30",
	Version: "2.0.30",
}

type releaseVersionMapping struct {
	spxVersion     string
	runtimeVersion string
}

var releaseVersionMappings = []releaseVersionMapping{
	{"v2.0.0-pre.28", "2.1.26"},
	{"v2.0.0-pre.29", "2.1.26"},
	{"v2.0.0-pre.30", "2.1.27"},
	{"v2.0.0-pre.31", "2.1.28"},
	{"v2.0.0-pre.32", "2.1.29"},
	{"v2.0.0-pre.33", "2.1.31"},
	{"v2.0.0-pre.34", "2.1.32"},
	{"v2.0.0-pre.35", "2.1.33"},
	{"v2.0.0-pre.36", "2.1.34"},
	{"v2.0.0-pre.37", "2.1.35"},
	{"v2.0.0-pre.38", "2.1.36"},
	{"v2.0.0-pre.39", "2.1.37"},
	{"v2.0.0-pre.40", "2.1.38"},
	{"v2.0.0-pre.41", "2.1.38"},
	{"v2.0.0-pre.42", "2.1.39"},
	{"v2.0.0-pre.43", "2.1.39"},
	{"v2.0.0-pre.44", "2.1.39"},
	{"v2.0.0-pre.45", "2.1.40"},
	{"v2.0.0-pre.46", "2.1.42"},
	{"v2.0.0-pre.47", "2.1.43"},
	{"v2.0.0-pre.48", "2.1.45"},
	{"v2.0.0-pre.49", "2.1.46"},
}

func newReleaseMeta(spxVersion, runtimeVersion string) ReleaseMeta {
	return ReleaseMeta{
		SPXVersion: spxVersion,
		Runtime: RuntimeRelease{
			Version: runtimeVersion,
		},
		Pck: defaultPckRelease,
	}
}

var releaseMetaBySPXVersion = func() map[string]ReleaseMeta {
	result := make(map[string]ReleaseMeta, len(releaseVersionMappings))
	for _, item := range releaseVersionMappings {
		result[item.spxVersion] = newReleaseMeta(item.spxVersion, item.runtimeVersion)
	}
	return result
}()

// DefaultReleaseMeta returns the latest known released metadata.
func DefaultReleaseMeta() ReleaseMeta {
	if len(releaseVersionMappings) == 0 {
		panic("releasemeta: releaseVersionMappings is empty")
	}
	item := releaseVersionMappings[len(releaseVersionMappings)-1]
	return newReleaseMeta(item.spxVersion, item.runtimeVersion)
}

// CurrentReleaseMeta is kept as a compatibility alias for the default release metadata.
func CurrentReleaseMeta() ReleaseMeta {
	return DefaultReleaseMeta()
}

// ReleaseMetaForSPXVersion resolves runtime/pck metadata for an SPX module version.
// Unknown versions fall back to the latest known runtime assets.
func ReleaseMetaForSPXVersion(spxVersion string) ReleaseMeta {
	if meta, ok := releaseMetaBySPXVersion[spxVersion]; ok {
		return meta
	}
	return DefaultReleaseMeta()
}

// RuntimeBinaryTag returns the runtime executable/pck base filename.
func (m ReleaseMeta) RuntimeBinaryTag() string {
	return RuntimeTag + m.Runtime.Version
}

// RuntimeDownloadURL returns the runtime archive URL for the given zip asset name.
func (m ReleaseMeta) RuntimeDownloadURL(zipName string) string {
	return RuntimeURLBase + "spx" + m.Runtime.Version + "/" + zipName
}

// PckDownloadURL returns the packaged runtime asset URL for the given zip asset name.
func (m ReleaseMeta) PckDownloadURL(zipName string) string {
	return SpxReleaseURLBase + m.Pck.SPXTag + "/" + zipName
}
