package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/goplus/spx/v4/internal/release"
)

type embeddedAssetManifest struct {
	CacheKey string   `json:"cache_key"`
	Names    []string `json:"names"`
}

func main() {
	// Keep the historical positional mode used by cmd/spx/install.sh. CI uses
	// the flag mode below to emit the reproducible release manifest.
	var err error
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		err = writeEmbeddedAssetManifest(os.Args[1:], os.Stdout)
	} else {
		err = runReleaseManifest(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runReleaseManifest(args []string) error {
	fs := flag.NewFlagSet("runtime-manifest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		lockPath, outputPath, checksumsPath  string
		revision, assetDir                   string
		verifyEngineManifest, verifyManifest string
		sourceManifest                       string
		provenance                           release.RuntimeProvenance
		assets                               []release.RuntimeAssetInput
	)
	fs.StringVar(&lockPath, "lock", "", "runtime lock JSON (defaults to the embedded lock)")
	fs.StringVar(&outputPath, "output", "-", "manifest output path, or - for stdout")
	fs.StringVar(&checksumsPath, "checksums", "", "optional SHA256SUMS output path")
	fs.StringVar(&revision, "revision", "", "derive all provenance from this Git revision instead of explicit provenance flags")
	fs.StringVar(&provenance.SPXCommit, "spx-commit", "", "full lowercase SPX commit SHA")
	fs.StringVar(&provenance.ModuleTree, "module-tree", "", "full lowercase SPX module tree SHA")
	fs.StringVar(&provenance.RuntimePackSourceSHA256, "runtime-pack-source-sha256", "", "SHA-256 of the tracked SPX runtime pack source inputs")
	fs.StringVar(&provenance.EngineSourceSHA256, "engine-source-sha256", "", "SHA-256 of the independent engine inputs")
	fs.StringVar(&provenance.BuildRecipeSHA256, "build-recipe-sha256", "", "SHA-256 of the tracked runtime build recipe")
	fs.StringVar(&verifyEngineManifest, "verify-engine-manifest", "", "verify an engine reuse source against its saved lock and current inputs")
	fs.StringVar(&sourceManifest, "engine-manifest", "", "verified source manifest for copied engine assets")
	fs.StringVar(&verifyManifest, "verify-manifest", "", "verify an existing release manifest instead of generating one")
	fs.StringVar(&assetDir, "asset-directory", "", "optionally verify manifest assets below this directory")
	fs.Func("asset", "release asset in name=path form; repeat for every required asset", func(spec string) error {
		asset, err := parseAsset(spec)
		if err != nil {
			return err
		}
		assets = append(assets, asset)
		return nil
	})
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: runtime-manifest [flags] --asset name=path ...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if err := validateManifestFlags(fs); err != nil {
		return err
	}
	lock := release.DefaultRuntimeLock()
	if lockPath != "" {
		var err error
		lock, err = release.LoadRuntimeLock(lockPath)
		if err != nil {
			return err
		}
	}
	if verifyEngineManifest != "" {
		source, err := release.LoadRuntimeManifest(verifyEngineManifest)
		if err != nil {
			return err
		}
		if err := release.ValidateEngineReuse(source, lock, ".", revision); err != nil {
			return err
		}
		if assetDir != "" {
			return source.VerifyEngineFiles(assetDir)
		}
		return nil
	}
	if verifyManifest != "" {
		data, err := os.ReadFile(verifyManifest)
		if err != nil {
			return fmt.Errorf("read runtime manifest: %w", err)
		}
		manifest, err := release.ParseRuntimeManifestForRelease(data, lock.RuntimeVersion, lock.RequiredAssets)
		if err != nil {
			return err
		}
		if assetDir != "" {
			return manifest.VerifyFiles(assetDir)
		}
		return nil
	}
	if assetDir != "" {
		return errors.New("--asset-directory requires --verify-manifest or --verify-engine-manifest")
	}
	if len(assets) == 0 {
		fs.Usage()
		return errors.New("at least one --asset is required")
	}

	if revision != "" {
		var err error
		provenance, err = release.RuntimeProvenanceAt(".", revision, lock)
		if err != nil {
			return err
		}
	} else {
		provenance.GodotCommit = lock.Godot.Commit
		provenance.Toolchain = lock.Toolchain
	}
	manifest, err := release.GenerateRuntimeManifest(lock, provenance, assets)
	if err != nil {
		return err
	}

	if sourceManifest != "" {
		source, err := release.LoadRuntimeManifest(sourceManifest)
		if err != nil {
			return err
		}
		if err := manifest.InheritEngine(source); err != nil {
			return err
		}
	}

	if outputPath == "-" {
		data, err := manifest.JSON()
		if err != nil {
			return err
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("write runtime manifest: %w", err)
		}
	} else if err := release.WriteRuntimeManifest(outputPath, manifest); err != nil {
		return err
	}
	if checksumsPath != "" {
		return release.WriteSHA256SUMS(checksumsPath, manifest)
	}
	return nil
}

func writeEmbeddedAssetManifest(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: runtime-manifest name=path [name=path ...]")
	}
	hasher := sha256.New()
	manifest := embeddedAssetManifest{Names: make([]string, 0, len(args))}
	for _, spec := range args {
		asset, err := parseAsset(spec)
		if err != nil {
			return err
		}
		if err := hashEmbeddedAsset(hasher, asset.Name, asset.Path); err != nil {
			return fmt.Errorf("hash asset %s: %w", asset.Name, err)
		}
		manifest.Names = append(manifest.Names, asset.Name)
	}
	manifest.CacheKey = hex.EncodeToString(hasher.Sum(nil))[:16]
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return nil
}

func parseAsset(spec string) (release.RuntimeAssetInput, error) {
	name, path, ok := strings.Cut(spec, "=")
	if !ok || name == "" || path == "" {
		return release.RuntimeAssetInput{}, fmt.Errorf("invalid asset %q: want non-empty name=path", spec)
	}
	return release.RuntimeAssetInput{Name: name, Path: path}, nil
}

func validateManifestFlags(fs *flag.FlagSet) error {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	explicitProvenance := set["spx-commit"] || set["module-tree"] || set["runtime-pack-source-sha256"] || set["build-recipe-sha256"] || set["engine-source-sha256"]
	if set["revision"] && (explicitProvenance || set["verify-manifest"]) {
		return errors.New("--revision cannot be combined with explicit provenance flags or --verify-manifest")
	}
	for _, name := range []string{"revision", "verify-manifest", "verify-engine-manifest", "engine-manifest"} {
		if set[name] && strings.TrimSpace(fs.Lookup(name).Value.String()) == "" {
			return fmt.Errorf("--%s must not be empty", name)
		}
	}
	if set["verify-manifest"] || set["verify-engine-manifest"] {
		if explicitProvenance || set["engine-manifest"] || set["asset"] || set["checksums"] || set["output"] {
			return errors.New("generation flags cannot be combined with manifest verification")
		}
	}
	if set["verify-engine-manifest"] && (!set["revision"] || set["verify-manifest"]) {
		return errors.New("--verify-engine-manifest requires --revision and cannot be combined with --verify-manifest")
	}
	if set["engine-manifest"] && !set["revision"] {
		return errors.New("--engine-manifest requires --revision")
	}
	return nil
}

func hashEmbeddedAsset(hasher io.Writer, name, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.WriteString(hasher, name+"\x00"); err != nil {
		return err
	}
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	_, err = io.WriteString(hasher, "\x00")
	return err
}
