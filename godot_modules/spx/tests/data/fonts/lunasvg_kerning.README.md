`lunasvg_kerning.ttf` is a test-only `A`/`V` subset of the SPX project
template font at `cmd/spx/template/project/engine/fonts/default.ttf`.

It was generated with HarfBuzz 12.2.0 using:

```text
hb-subset --no-hinting --glyph-names --text=AV \
  --output-file=lunasvg_kerning.ttf default.ttf
```

The template font is itself a modified Latin subset of Source Han Sans CN
Medium 1.000. The original font is copyright (c) 2014 Adobe Systems
Incorporated. Its upstream source is
https://github.com/adobe-fonts/source-han-sans/tree/1.000 and the SPX source
artifact is published at
https://github.com/goplus/godot/releases/download/spx2.0.14/CnFont.ttf.

The font and this further subset are licensed under Apache License 2.0. See
`lunasvg_kerning.LICENSE.txt` and `lunasvg_kerning.NOTICE.txt`.
