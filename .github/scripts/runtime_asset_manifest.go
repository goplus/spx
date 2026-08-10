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

	"github.com/goplus/spx/v3/internal/release"
)

type assetManifest struct {
	CacheKey string   `json:"cache_key"`
	Names    []string `json:"names"`
}

type assetSpecs []string

func (specs *assetSpecs) String() string {
	return strings.Join(*specs, ",")
}

func (specs *assetSpecs) Set(value string) error {
	if _, _, ok := strings.Cut(value, "="); !ok {
		return fmt.Errorf("invalid asset spec %q: want name=path", value)
	}
	*specs = append(*specs, value)
	return nil
}

func main() {
	// Keep the historical positional mode used by cmd/spx/install.sh. CI uses
	// the flag mode below to emit the reproducible release manifest.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		if err := writeEmbeddedAssetManifest(os.Args[1:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := runReleaseManifest(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runReleaseManifest(args []string) error {
	fs := flag.NewFlagSet("runtime_asset_manifest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		lockPath              string
		outputPath            string
		checksumsPath         string
		spxCommit             string
		moduleTree            string
		runtimePackSourceHash string
		buildRecipeHash       string
		verifyManifest        string
		assetDirectory        string
		specs                 assetSpecs
	)
	fs.StringVar(&lockPath, "lock", "", "runtime lock JSON (defaults to the embedded lock)")
	fs.StringVar(&outputPath, "output", "-", "manifest output path, or - for stdout")
	fs.StringVar(&checksumsPath, "checksums", "", "optional SHA256SUMS output path")
	fs.StringVar(&spxCommit, "spx-commit", "", "full lowercase SPX commit SHA")
	fs.StringVar(&moduleTree, "module-tree", "", "full lowercase SPX module tree SHA")
	fs.StringVar(&runtimePackSourceHash, "runtime-pack-source-sha256", "", "SHA-256 of the tracked SPX runtime pack source inputs")
	fs.StringVar(&buildRecipeHash, "build-recipe-sha256", "", "SHA-256 of the tracked runtime build recipe")
	fs.StringVar(&verifyManifest, "verify-manifest", "", "verify an existing release manifest instead of generating one")
	fs.StringVar(&assetDirectory, "asset-directory", "", "optionally verify manifest assets below this directory")
	fs.Var(&specs, "asset", "release asset in name=path form; repeat for every required asset")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: runtime_asset_manifest [flags] --asset name=path ...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	lock := release.DefaultRuntimeLock()
	if lockPath != "" {
		data, err := os.ReadFile(lockPath)
		if err != nil {
			return fmt.Errorf("read runtime lock: %w", err)
		}
		parsed, err := release.ParseRuntimeLock(data)
		if err != nil {
			return err
		}
		lock = parsed
	}
	if verifyManifest != "" {
		if len(specs) != 0 || checksumsPath != "" || spxCommit != "" || moduleTree != "" || runtimePackSourceHash != "" || buildRecipeHash != "" {
			return errors.New("generation flags cannot be combined with --verify-manifest")
		}
		manifest, err := release.LoadRuntimeManifest(verifyManifest)
		if err != nil {
			return err
		}
		if err := manifest.ValidateForLock(lock); err != nil {
			return err
		}
		if assetDirectory != "" {
			return manifest.VerifyFiles(assetDirectory)
		}
		return nil
	}
	if assetDirectory != "" {
		return errors.New("--asset-directory requires --verify-manifest")
	}
	if len(specs) == 0 {
		fs.Usage()
		return errors.New("at least one --asset is required")
	}

	inputs := make([]release.RuntimeAssetInput, 0, len(specs))
	for _, spec := range specs {
		name, path, _ := strings.Cut(spec, "=")
		if name == "" || path == "" {
			return fmt.Errorf("invalid asset spec %q: want non-empty name=path", spec)
		}
		inputs = append(inputs, release.RuntimeAssetInput{Name: name, Path: path})
	}
	manifest, err := release.GenerateRuntimeManifest(lock, release.RuntimeProvenance{
		SPXCommit:               spxCommit,
		GodotCommit:             lock.Godot.Commit,
		ModuleTree:              moduleTree,
		RuntimePackSourceSHA256: runtimePackSourceHash,
		BuildRecipeSHA256:       buildRecipeHash,
		Toolchain:               lock.Toolchain,
	}, inputs)
	if err != nil {
		return err
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
		if err := release.WriteSHA256SUMS(checksumsPath, manifest); err != nil {
			return err
		}
	}
	return nil
}

func writeEmbeddedAssetManifest(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: runtime_asset_manifest name=path [name=path ...]")
	}
	hasher := sha256.New()
	manifest := assetManifest{Names: make([]string, 0, len(args))}
	for _, arg := range args {
		name, path, ok := strings.Cut(arg, "=")
		if !ok || name == "" || path == "" {
			return fmt.Errorf("invalid asset spec: %q", arg)
		}
		if err := hashEmbeddedAsset(hasher, name, path); err != nil {
			return fmt.Errorf("hash asset %s: %w", name, err)
		}
		manifest.Names = append(manifest.Names, name)
	}
	manifest.CacheKey = hex.EncodeToString(hasher.Sum(nil))[:16]
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
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
