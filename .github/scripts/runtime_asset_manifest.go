package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type assetManifest struct {
	CacheKey string   `json:"cache_key"`
	Names    []string `json:"names"`
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintln(os.Stderr, "usage: runtime_asset_manifest name=path [name=path ...]")
		os.Exit(1)
	}

	hasher := sha256.New()
	manifest := assetManifest{
		Names: make([]string, 0, len(os.Args)-1),
	}

	for _, arg := range os.Args[1:] {
		name, path, ok := strings.Cut(arg, "=")
		if !ok || name == "" || path == "" {
			fmt.Fprintf(os.Stderr, "invalid asset spec: %q\n", arg)
			os.Exit(1)
		}
		if err := hashAsset(hasher, name, path); err != nil {
			fmt.Fprintf(os.Stderr, "hash asset %s: %v\n", name, err)
			os.Exit(1)
		}
		manifest.Names = append(manifest.Names, name)
	}

	manifest.CacheKey = hex.EncodeToString(hasher.Sum(nil))[:16]

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "encode manifest: %v\n", err)
		os.Exit(1)
	}
}

func hashAsset(hasher io.Writer, name, path string) error {
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
