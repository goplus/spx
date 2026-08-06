# ProjectFonts

This is the single end-to-end example for project Font Collections and SVG font fallback.

- There are no built-in Scratch family registrations in the runtime.
- All seven Scratch-compatible families, `basic-chinese`, the Arabic shaping subset, and the complete `Color Emoji` family live under `assets/fonts`.
- Each family directory contains exactly one `faces` entry.
- The backdrop compares explicit project families, verifies the global `Pixel, Color Emoji, Arabic, basic-chinese, default` fallback order, and keeps an explicit `Scratch, basic-chinese` SVG list to verify that SVG preferences override the global list.
- A text element that names only the unavailable `Missing Family` renders one missing-glyph box instead of falling back to the global preferences.
- The built-in `default` family is Latin-only; all Chinese text is supplied by the Han-only `basic-chinese` subset.
- Emoji, Chinese, combining-mark, variation-selector and ZWJ clusters exercise cluster-atomic fallback and HarfBuzz sequence shaping.
- `SPX 123 | سلام | 456 END` remains in logical source order. Its expected visual order is `SPX 123 | 456 | سلام END`, with connected RTL Arabic glyphs, exercising ICU Unicode BiDi run ordering plus Arabic contextual joining through the project `Arabic` family.

From the repository root, run `spx run --path test/ProjectFonts`.
