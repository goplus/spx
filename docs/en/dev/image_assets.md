# Static WebP image assets

SPX accepts static WebP files anywhere a raster texture is accepted, including backdrops, individual costumes, animation sequence frames, and atlas source images. Ordinary frame sequences may mix WebP with PNG or JPEG files.

An animated WebP bitstream is not treated as an SPX animation. Keep animation timing and frame ranges in the SPX project configuration, using either individual static frames or an atlas.

Renaming a PNG or JPEG file to use a `.webp` extension does not convert it. Encode the image as WebP and keep the file extension consistent with its actual contents; otherwise the texture loader reports the file as corrupt.

Use lossless WebP for pixel art, sharp transparent edges, and costumes that participate in pixel-perfect collision checks. Lossy WebP is better suited to photographic artwork where small color changes are acceptable.

WebP can reduce project and download size, but it does not reduce decoded texture memory. SPX may decode and cache frame textures during sprite initialization or animation registration, so large sequences should be checked on representative Web and low-end devices.
