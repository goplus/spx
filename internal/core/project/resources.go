package project

import (
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"syscall"

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func AssetDirFromResource(resource any) (string, bool) {
	switch v := resource.(type) {
	case string:
		return v, true
	case spxfs.GdDir:
		return strings.TrimSuffix(v.GetPath(), "/"), true
	default:
		return "", false
	}
}

func ResourceDir(resource any) (spxfs.Dir, error) {
	if fs, ok := resource.(spxfs.Dir); ok {
		return fs, nil
	}
	return spxfs.Open(resource.(string))
}

type OpenedBuilderResources struct {
	AssetDir string
	FS       spxfs.Dir
	LoadedBuilderProject
}

func OpenBuilderResources(resource any, gameConf *Config) (OpenedBuilderResources, error) {
	var opened OpenedBuilderResources
	opened.AssetDir, _ = AssetDirFromResource(resource)

	fs, err := ResourceDir(resource)
	if err != nil {
		return OpenedBuilderResources{}, err
	}
	opened.FS = fs

	loaded, err := LoadBuilderProject(fs, gameConf)
	if err != nil {
		return OpenedBuilderResources{}, err
	}
	opened.LoadedBuilderProject = loaded
	return opened, nil
}

func LoadJSON(ret any, fs spxfs.Dir, file string) error {
	if _, ok := fs.(spxfs.GdDir); ok {
		filePath := engine.ToAssetPath(file)
		if engine.HasFile(filePath) {
			value := engine.ReadAllText(filePath)
			return json.Unmarshal([]byte(value), ret)
		}
		return errors.New("error : Load json failed,file not exit " + filePath)
	}

	f, err := fs.Open(file)
	if err != nil {
		spxlog.Error("failed to open file %s: %v", file, err)
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(ret)
}

func LoadConfig(ret any, fs spxfs.Dir, index any) error {
	switch v := index.(type) {
	case io.Reader:
		return json.NewDecoder(v).Decode(ret)
	case string:
		return LoadJSON(ret, fs, v)
	case nil:
		return LoadJSON(ret, fs, "index.json")
	default:
		return syscall.EINVAL
	}
}

func normalizeConfigPath(configDir, relPath string) string {
	if relPath == "" {
		return ""
	}
	if strings.HasPrefix(relPath, "/") {
		return relPath
	}
	if schema, _ := spxfs.SplitSchema(relPath); schema != "" {
		return relPath
	}
	return path.Clean(path.Join(configDir, relPath))
}

func normalizeProjectConfigPaths(conf *ProjectConfig) {
	if conf == nil {
		return
	}

	for _, backdrop := range conf.Backdrops {
		if backdrop == nil {
			continue
		}
		backdrop.Path = normalizeConfigPath("", backdrop.Path)
	}
	conf.Bgm = normalizeConfigPath("", conf.Bgm)
	conf.TilemapPath = normalizeConfigPath("", conf.TilemapPath)
}

func normalizeSpriteConfigPaths(conf *SpriteConfig, configDir string) {
	if conf == nil {
		return
	}

	for _, costume := range conf.Costumes {
		if costume == nil {
			continue
		}
		costume.Path = normalizeConfigPath(configDir, costume.Path)
	}
	if conf.CostumeSet != nil {
		conf.CostumeSet.Path = normalizeConfigPath(configDir, conf.CostumeSet.Path)
	}
	if conf.CostumeMPSet != nil {
		conf.CostumeMPSet.Path = normalizeConfigPath(configDir, conf.CostumeMPSet.Path)
	}
}

type LoadedSpriteConfig struct {
	BaseDir string
	Config  SpriteConfig
}

func LoadSpriteConfig(fs spxfs.Dir, name string) (LoadedSpriteConfig, error) {
	baseDir := path.Join("sprites", name) + "/"
	var conf SpriteConfig
	if err := LoadJSON(&conf, fs, baseDir+"index.json"); err != nil {
		return LoadedSpriteConfig{}, err
	}
	normalizeSpriteConfigPaths(&conf, strings.TrimSuffix(baseDir, "/"))
	return LoadedSpriteConfig{
		BaseDir: baseDir,
		Config:  conf,
	}, nil
}

type LoadedSoundConfig struct {
	BaseDir string
	Config  SoundConfig
}

func LoadSoundConfig(fs spxfs.Dir, name string) (LoadedSoundConfig, error) {
	baseDir := path.Join("sounds", name)
	var conf SoundConfig
	if err := LoadJSON(&conf, fs, path.Join(baseDir, "index.json")); err != nil {
		return LoadedSoundConfig{}, err
	}
	conf.Path = normalizeConfigPath(baseDir, conf.Path)
	return LoadedSoundConfig{
		BaseDir: baseDir,
		Config:  conf,
	}, nil
}
