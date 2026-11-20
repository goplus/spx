# Project assets

## Resource names and paths in project indexes

SPX looks for `index_pack.json` in the asset root (usually `assets/`) when opening a project. It can contain the project settings and the `sprites`, `sounds`, and `fonts` sections. [Example packed index](../../../test/IndexPackMode/assets/index_pack.json).

If both `index.json` and `index_pack.json` exist, the packed index takes precedence for matching top-level fields. Each sprite or sound uses its packed entry when present and its own `index.json` otherwise. A `fonts` field in the packed index defines the entire font catalog, even when empty. SPX scans `assets/fonts/` only if that field is absent.

Project producers such as XBuilder and sb2xbp must validate names before creating sprite, sound, or font family directories. Each name must be one non-empty POSIX path segment. The values `.` and `..`, a slash (`/`), and NUL are invalid. Spaces and Unicode are valid. A backslash or colon is a literal character in a POSIX name. For example, `"bgm": "主题曲"` selects the sound defined by `sounds/主题曲/index.json`. Its `path` field identifies the audio file.

Resource references use `/` and resolve from the source configuration’s directory. A packed entry uses the same directory as its source equivalent. For example, `../../shared/image.png` resolves to `assets/shared/image.png` in both `assets/sprites/Hero/index.json` and the `sprites.Hero` entry of `index_pack.json`. To migrate a project that used `extasset` in `.config`, move the shared files under `assets/` and update their references. Absolute paths such as `/a/b` and `C:/a/b`, UNC paths, URIs such as `res://`, and references outside the selected asset root are rejected. A font face must also stay inside its family directory.

To migrate code that passed an external path such as `../../tile.png` to `PlaceTile` or `PlaceTiles`, move the image to `assets/tiles/tile.png` and pass `tiles/tile.png`. Go hosts that called `ispx.ConfigureLegacyFilesystemRoots` must switch to `ispx.ConfigureFilesystemRoots` and keep their runtime resources inside the project root.

### How each platform reads assets

| Environment | Resource location and lookup |
| --- | --- |
| Linux and macOS desktop | The source project reads and lists files under the selected `assets/` directory. When started from the generated `project/` directory with no local `assets/`, it uses `../assets/`. A native POSIX source index can reference a file literally named `res:image.png`. |
| Windows desktop | Asset root selection is the same as on other desktops. Project configurations use `/`. The runtime converts the path when accessing the disk. Colons are invalid in Windows filenames. |
| Web preview and export (`ispx`) | The interpreter reads source and JSON from memory. Godot reads images and audio from the mounted project files. A costume path of `res:image.png` refers to a file with that name in the sprite directory. |
| Android and iOS | The mobile asset reader uses the fixed `assets` root. Android reads bundled assets through AssetManager and iOS reads from the app bundle. Exported games can also load Godot resources through `packmode`. |

Direct desktop reads resolve symlinks and check that the target stays under the asset root. The desktop interpreter rejects symlink components. In an export built with `packmode`, the engine maps relative references to `res://assets/…` for the default asset root. Project JSON still uses relative paths. Desktop filename matching follows the host volume. The SPX ZIP reader matches entry names exactly, so use the same spelling and case in the configuration and on disk. Godot interprets backslashes as separators. Avoid backslashes and colons in physical filenames intended for multiple platforms.

## Static WebP image assets

SPX accepts static WebP files anywhere a raster texture is accepted, including backdrops, individual costumes, animation sequence frames, and atlas source images. Ordinary frame sequences may mix WebP with PNG or JPEG files.

An animated WebP bitstream is not treated as an SPX animation. Keep animation timing and frame ranges in the SPX project configuration, using either individual static frames or an atlas.

Renaming a PNG or JPEG file to use a `.webp` extension does not convert it. Encode the image as WebP and keep the file extension consistent with its actual contents; otherwise the texture loader reports the file as corrupt.

Use lossless WebP for pixel art, sharp transparent edges, and costumes that participate in pixel-perfect collision checks. Lossy WebP is better suited to photographic artwork where small color changes are acceptable.

WebP can reduce project and download size, but it does not reduce decoded texture memory. SPX may decode and cache frame textures during sprite initialization or animation registration, so large sequences should be checked on representative Web and low-end devices.
