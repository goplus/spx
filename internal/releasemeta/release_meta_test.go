package releasemeta

import "testing"

func TestReleaseMetaForSPXVersionMapped(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("v2.0.0-pre.30")
	if meta.Runtime.Version != "2.1.27" {
		t.Fatalf("runtime version = %q, want %q", meta.Runtime.Version, "2.1.27")
	}
	if meta.Pck.SPXTag != "v2.0.0-pre.30" {
		t.Fatalf("pck tag = %q, want %q", meta.Pck.SPXTag, "v2.0.0-pre.30")
	}
	if meta.Pck.Version != "2.0.30" {
		t.Fatalf("pck version = %q, want %q", meta.Pck.Version, "2.0.30")
	}
}

func TestReleaseMetaForSPXVersionLatestFallback(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("latest")
	defaultMeta := DefaultReleaseMeta()
	if meta.SPXVersion != defaultMeta.SPXVersion {
		t.Fatalf("spx version = %q, want default %q", meta.SPXVersion, defaultMeta.SPXVersion)
	}
	if meta.Runtime.Version != defaultMeta.Runtime.Version {
		t.Fatalf("runtime version = %q, want default %q", meta.Runtime.Version, defaultMeta.Runtime.Version)
	}
}

func TestReleaseMetaForSPXVersionUnknownFallback(t *testing.T) {
	meta := ReleaseMetaForSPXVersion("v2.0.0-pre.99")
	defaultMeta := DefaultReleaseMeta()
	if meta.SPXVersion != defaultMeta.SPXVersion {
		t.Fatalf("spx version = %q, want default %q", meta.SPXVersion, defaultMeta.SPXVersion)
	}
	if meta.Runtime.Version != defaultMeta.Runtime.Version {
		t.Fatalf("runtime version = %q, want default %q", meta.Runtime.Version, defaultMeta.Runtime.Version)
	}
}
