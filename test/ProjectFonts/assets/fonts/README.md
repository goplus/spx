Fonts used by the `test/ProjectFonts` project Font Collection:

- `NotoSans-Medium.ttf` -> `Sans Serif`
- `SourceSerifPro-Regular.otf` -> `Serif`
- `handlee-regular.ttf` -> `Handwriting`
- `Knewave.ttf` -> `Marker`
- `Griffy-Regular.ttf` -> `Curly`
- `Grand9K-Pixel.ttf` -> `Pixel`
- `Scratch.ttf` -> `Scratch`
- `basic-chinese.ttf` -> `basic-chinese` (Han/CJK subset without Latin glyphs)
- `TwitterColorEmoji-SVGinOT.ttf` -> `Color Emoji`

Source:

- https://github.com/scratchfoundation/scratch-render-fonts
- https://github.com/13rac1/twemoji-color-font
- https://github.com/goplus/godot/releases/download/spx2.0.14/CnFont.ttf
- https://github.com/adobe-fonts/source-han-sans/tree/1.000

`basic-chinese.ttf` is subset from `CnFont.ttf`. It retains Han ideographs,
CJK radicals and strokes, Chinese and fullwidth punctuation, and compatibility
ideographs while excluding Latin glyphs.

Each family is declared by its own `assets/fonts/<Family>/index.json`. Licenses:

- See `LICENSE.txt`
- See `OFL.txt`
- See `basic-chinese/basic-chinese.LICENSE.txt`
- See `basic-chinese/basic-chinese.NOTICE.txt`
- See the license files under `Color Emoji/`
