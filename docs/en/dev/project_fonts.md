# Project Fonts and SVG Cluster Fallback

## Goals

SPX supports project-provided fonts while preserving reliable fallback for multilingual text, emoji, and SVG text. Font updates are applied atomically so a frame never observes a partially rebuilt fallback chain.

## Font collection

A project can declare an ordered collection of preferred fonts. Order matters: the first font that can render a text cluster is selected. Project fonts are resolved from project assets and coexist with bundled fallback fonts.

## Loading and validation

The runtime loads every configured font, validates it, and builds a replacement collection before publishing the new configuration. If required input is invalid, the previous valid configuration remains active instead of exposing a partial update.

## Runtime flow

1. Read the project font preferences.
2. Resolve and load font resources.
3. Construct the ordered project collection.
4. Append the required SPX fallback fonts.
5. Atomically replace the active collection.
6. Invalidate affected text layout and rendering state.

## SVG family and cluster fallback

SVG text may request a font family that is unavailable or lacks part of a string. Fallback is selected for a shaped text cluster rather than blindly for each byte or code unit. This matters for Arabic shaping, combining marks, variation selectors, and emoji sequences.

The requested SVG family remains a preference, not a guarantee. SPX falls back through the project collection and then bundled defaults when the requested family cannot render the complete cluster.

## Reset behavior

Resetting project fonts removes project-specific preferences and restores the default SPX collection. Reset must also invalidate cached layout so existing text nodes no longer retain stale font references.

## Default fonts

SPX keeps general text, Chinese text, symbols, and emoji fallback responsibilities separate. Bundled fallback assets are intentionally limited; applications that need broad script or emoji coverage should ship suitable project fonts.

## Example

Test coverage and sample assets are available in [`test/ProjectFonts`](../../../test/ProjectFonts). The sample includes Latin, Arabic, and color emoji resources and demonstrates ordered preferences and fallback behavior.

## Current limitations

- Font availability depends on the assets packaged with the project and runtime.
- SVG/CSS font semantics are not a complete browser implementation.
- Complex-script quality depends on shaping support and the selected font.
- A fallback font must cover the complete cluster to avoid broken glyph composition.
