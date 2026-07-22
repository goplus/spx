# ProjectFonts

This is the single end-to-end example for project Font Collections and SVG font fallback.

- There are no built-in Scratch family registrations in the runtime.
- All seven Scratch-compatible families and the complete Twitter Color Emoji font live under `assets/fonts`.
- Each family directory contains exactly one `faces` entry.
- The backdrop compares explicit project families and verifies the global `Pixel, Heart Emoji, default` fallback order.
- Emoji, Chinese, combining-mark, variation-selector and ZWJ clusters exercise cluster-atomic fallback and HarfBuzz sequence shaping.

From the repository root, run `spx run --path test/ProjectFonts`.
