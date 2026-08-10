These fonts are test-only subsets of the hinted Noto Sans fonts from the
archived `notofonts/noto-fonts` distribution:

- https://github.com/notofonts/noto-fonts/tree/main/hinted/ttf/NotoSansArabic
- https://github.com/notofonts/noto-fonts/tree/main/hinted/ttf/NotoSansHebrew
- https://github.com/notofonts/noto-fonts/tree/main/hinted/ttf/NotoSansDevanagari
- https://github.com/notofonts/noto-fonts/tree/main/hinted/ttf/NotoSansThai

They were reduced with `hb-subset` 12.2.0 to the characters used by the
LunaSVG complex-text tests. HarfBuzz layout closure was retained, so joining,
ligature, reordering, and mark-positioning glyphs required by those strings
remain in the files.

The original fonts and these subsets are distributed under the SIL Open Font
License 1.1 in `LICENSE.txt`.
