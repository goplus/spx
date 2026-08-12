#include "svgtextelement.h"
#include "svglayoutstate.h"
#include "svgrenderstate.h"
#include <lunasvg.h>

#include <algorithm>
#include <cassert>
#include <cmath>
#include <memory>
#include <unordered_map>
#include <utility>

#ifdef LUNASVG_ENABLE_HARFBUZZ
#include <hb-ot.h>
#include <hb.h>
#endif
#ifdef LUNASVG_ENABLE_ICU_BIDI
#include <unicode/ubidi.h>
#endif

namespace lunasvg {

static thread_local GraphemeBreakFunction graphemeBreakFunction = nullptr;
static thread_local void *graphemeBreakClosure = nullptr;
static thread_local ShapingObserverFunction shapingObserverFunction = nullptr;
static thread_local void *shapingObserverClosure = nullptr;

void setGraphemeBreakFunction(GraphemeBreakFunction callback, void *closure) {
	graphemeBreakFunction = callback;
	graphemeBreakClosure = closure;
}

void setShapingObserverFunction(ShapingObserverFunction callback, void *closure) {
	shapingObserverFunction = callback;
	shapingObserverClosure = closure;
}

namespace {

struct GraphemeCluster {
	size_t start = 0;
	size_t end = 0;
	bool isWhitespace = false;
};

static bool isWhitespaceCodepoint(uint32_t codepoint) {
	switch (codepoint) {
		case 0x0009:
		case 0x000A:
		case 0x000B:
		case 0x000C:
		case 0x000D:
		case 0x0020:
		case 0x0085:
		case 0x00A0:
		case 0x1680:
		case 0x2028:
		case 0x2029:
		case 0x202F:
		case 0x205F:
		case 0x3000:
			return true;
		default:
			break;
	}

	return (codepoint >= 0x2000 && codepoint <= 0x200A);
}

struct EmbeddedSVGGlyphInfo {
	const char *svgData = nullptr;
	size_t svgLength = 0;
	Rect dstRect;
};

struct EmbeddedSVGGlyphBitmapCacheKey {
	const plutovg_font_face_t *face = nullptr;
	uint32_t glyphIndex = 0;

	bool operator==(const EmbeddedSVGGlyphBitmapCacheKey &other) const {
		return face == other.face && glyphIndex == other.glyphIndex;
	}
};

struct EmbeddedSVGGlyphBitmapCacheKeyHash {
	size_t operator()(const EmbeddedSVGGlyphBitmapCacheKey &key) const {
		auto faceHash = std::hash<const void *>{}(key.face);
		auto glyphHash = std::hash<uint32_t>{}(key.glyphIndex);
		return faceHash ^ (glyphHash + 0x9e3779b9 + (faceHash << 6) + (faceHash >> 2));
	}
};

using EmbeddedSVGGlyphBitmapCache = std::unordered_map<EmbeddedSVGGlyphBitmapCacheKey, Bitmap, EmbeddedSVGGlyphBitmapCacheKeyHash>;

static EmbeddedSVGGlyphBitmapCache &embeddedSVGGlyphBitmapCache();

static bool fontFaceHasGlyph(const FontFace &face, uint32_t codepoint) {
	return !face.isNull() && plutovg_font_face_has_glyph(face.get(), codepoint);
}

static bool isVariationSelector(uint32_t codepoint) {
	return (codepoint >= 0xFE00 && codepoint <= 0xFE0F) || (codepoint >= 0xE0100 && codepoint <= 0xE01EF);
}

static bool isClusterControl(uint32_t codepoint) {
	return codepoint == 0x200C || codepoint == 0x200D || isVariationSelector(codepoint) ||
			(codepoint >= 0xE0020 && codepoint <= 0xE007F);
}

static bool isGraphemeExtend(uint32_t codepoint) {
	return isVariationSelector(codepoint) || (codepoint >= 0x0300 && codepoint <= 0x036F) ||
			(codepoint >= 0x1AB0 && codepoint <= 0x1AFF) || (codepoint >= 0x1DC0 && codepoint <= 0x1DFF) ||
			(codepoint >= 0x20D0 && codepoint <= 0x20FF) || (codepoint >= 0xFE20 && codepoint <= 0xFE2F) ||
			(codepoint >= 0x1F3FB && codepoint <= 0x1F3FF) || (codepoint >= 0xE0020 && codepoint <= 0xE007F);
}

static bool isIndicVirama(uint32_t codepoint) {
	switch (codepoint) {
		case 0x094D: // Devanagari
		case 0x09CD: // Bengali
		case 0x0A4D: // Gurmukhi
		case 0x0ACD: // Gujarati
		case 0x0B4D: // Oriya
		case 0x0BCD: // Tamil
		case 0x0C4D: // Telugu
		case 0x0CCD: // Kannada
		case 0x0D3B: // Malayalam vertical bar virama
		case 0x0D3C: // Malayalam circular virama
		case 0x0D4D: // Malayalam
		case 0x0DCA: // Sinhala
		case 0x1039: // Myanmar
		case 0x103A:
		case 0x1714: // Tagalog
		case 0x1734: // Hanunoo
		case 0x17D2: // Khmer
		case 0x1A60: // Tai Tham
		case 0x1B44: // Balinese
		case 0x1BAA: // Sundanese
		case 0xA806: // Syloti Nagri
		case 0xA8C4: // Saurashtra
		case 0xA953: // Rejang
		case 0xAAF6: // Meetei Mayek
			return true;
		default:
			return false;
	}
}

static bool isGraphemeControl(uint32_t codepoint) {
	return codepoint == 0x000D || codepoint == 0x000A || codepoint == 0x2028 || codepoint == 0x2029 ||
			codepoint <= 0x001F || (codepoint >= 0x007F && codepoint <= 0x009F);
}

static bool isGraphemePrepend(uint32_t codepoint) {
	return (codepoint >= 0x0600 && codepoint <= 0x0605) || codepoint == 0x06DD || codepoint == 0x070F ||
			codepoint == 0x08E2 || codepoint == 0x0D4E || codepoint == 0x110BD || codepoint == 0x110CD ||
			(codepoint >= 0x111C2 && codepoint <= 0x111C3) || (codepoint >= 0x1193F && codepoint <= 0x1193F) ||
			codepoint == 0x11941 || codepoint == 0x11A3A || (codepoint >= 0x11A84 && codepoint <= 0x11A89) ||
			codepoint == 0x11D46;
}

enum class HangulSyllableType : uint8_t {
	None,
	L,
	V,
	T,
	LV,
	LVT,
};

static HangulSyllableType hangulSyllableType(uint32_t codepoint) {
	if ((codepoint >= 0x1100 && codepoint <= 0x115F) || (codepoint >= 0xA960 && codepoint <= 0xA97C)) {
		return HangulSyllableType::L;
	}
	if ((codepoint >= 0x1160 && codepoint <= 0x11A7) || (codepoint >= 0xD7B0 && codepoint <= 0xD7C6)) {
		return HangulSyllableType::V;
	}
	if ((codepoint >= 0x11A8 && codepoint <= 0x11FF) || (codepoint >= 0xD7CB && codepoint <= 0xD7FB)) {
		return HangulSyllableType::T;
	}
	if (codepoint >= 0xAC00 && codepoint <= 0xD7A3) {
		return (codepoint - 0xAC00) % 28 == 0 ? HangulSyllableType::LV : HangulSyllableType::LVT;
	}
	return HangulSyllableType::None;
}

static bool isExtendedPictographic(uint32_t codepoint) {
	return (codepoint >= 0x1F000 && codepoint <= 0x1FAFF) || (codepoint >= 0x2600 && codepoint <= 0x27BF) ||
			codepoint == 0x00A9 || codepoint == 0x00AE || codepoint == 0x203C || codepoint == 0x2049 ||
			codepoint == 0x2122 || codepoint == 0x2139 || codepoint == 0x3030 || codepoint == 0x303D ||
			codepoint == 0x3297 || codepoint == 0x3299;
}

static bool isRegionalIndicator(uint32_t codepoint) {
	return codepoint >= 0x1F1E6 && codepoint <= 0x1F1FF;
}

static std::vector<size_t> fallbackGraphemeBreaks(std::u32string_view text) {
	std::vector<size_t> breaks;
	size_t regionalIndicatorCount = !text.empty() && isRegionalIndicator(text.front()) ? 1 : 0;
	for (size_t i = 1; i < text.size(); ++i) {
		const auto previous = text[i - 1];
		const auto current = text[i];
		bool shouldBreak = true;
		if (previous == '\r' && current == '\n') {
			shouldBreak = false;
		} else if (isGraphemeControl(previous) || isGraphemeControl(current)) {
			shouldBreak = true;
		} else if (isGraphemePrepend(previous) || current == 0x200D || isGraphemeExtend(current) || isIndicVirama(current)) {
			shouldBreak = false;
		} else {
			const auto previousHangul = hangulSyllableType(previous);
			const auto currentHangul = hangulSyllableType(current);
			if (previousHangul == HangulSyllableType::L &&
					(currentHangul == HangulSyllableType::L || currentHangul == HangulSyllableType::V || currentHangul == HangulSyllableType::LV || currentHangul == HangulSyllableType::LVT)) {
				shouldBreak = false;
			} else if ((previousHangul == HangulSyllableType::LV || previousHangul == HangulSyllableType::V) &&
					(currentHangul == HangulSyllableType::V || currentHangul == HangulSyllableType::T)) {
				shouldBreak = false;
			} else if ((previousHangul == HangulSyllableType::LVT || previousHangul == HangulSyllableType::T) && currentHangul == HangulSyllableType::T) {
				shouldBreak = false;
			} else if (isIndicVirama(previous)) {
				// This is a conservative fallback for conjunct-forming scripts. The
				// host grapheme callback remains authoritative when available.
				shouldBreak = false;
			} else if (previous == 0x200D && isExtendedPictographic(current)) {
				size_t lookbehind = i - 1;
				while (lookbehind > 0 && isGraphemeExtend(text[lookbehind - 1])) {
					--lookbehind;
				}
				shouldBreak = lookbehind == 0 || !isExtendedPictographic(text[lookbehind - 1]);
			}
		}
		if (isRegionalIndicator(previous) && isRegionalIndicator(current)) {
			shouldBreak = regionalIndicatorCount % 2 == 0;
		}

		if (shouldBreak) {
			breaks.push_back(i);
			regionalIndicatorCount = 0;
		}
		if (isRegionalIndicator(current)) {
			++regionalIndicatorCount;
		} else if (!isGraphemeExtend(current) && current != 0x200D) {
			regionalIndicatorCount = 0;
		}
	}
	if (!text.empty()) {
		breaks.push_back(text.size());
	}
	return breaks;
}

static std::vector<size_t> graphemeBreaks(std::u32string_view text) {
	auto fallbackBreaks = fallbackGraphemeBreaks(text);
	if (text.empty() || graphemeBreakFunction == nullptr) {
		return fallbackBreaks;
	}

	std::vector<size_t> rawBreaks(text.size());
	auto count = graphemeBreakFunction(reinterpret_cast<const uint32_t *>(text.data()), text.size(), rawBreaks.data(),
			rawBreaks.size(), graphemeBreakClosure);
	if (count == 0) {
		return fallbackBreaks;
	}
	count = std::min(count, rawBreaks.size());
	std::vector<size_t> breaks;
	size_t previous = 0;
	for (size_t i = 0; i < count; ++i) {
		auto current = rawBreaks[i];
		if (current <= previous || current > text.size()) {
			return fallbackBreaks;
		}
		// Some host text servers report codepoint boundaries when a full Unicode
		// break iterator is unavailable. Never let those boundaries split a
		// cluster which the local UAX #29 subset knows must stay atomic (for
		// example combining marks, variation selectors, and emoji ZWJ sequences).
		if (std::binary_search(fallbackBreaks.begin(), fallbackBreaks.end(), current)) {
			breaks.push_back(current);
		}
		previous = current;
	}
	if (previous != text.size() || breaks.empty() || breaks.back() != text.size()) {
		return fallbackBreaks;
	}
	return breaks;
}

static std::u32string drawableClusterText(std::u32string_view cluster) {
	std::u32string result;
	for (auto codepoint : cluster) {
		if (!isClusterControl(codepoint)) {
			result.push_back(codepoint);
		}
	}
	return result;
}

static bool isWhitespaceCluster(std::u32string_view cluster) {
	for (auto codepoint : cluster) {
		if (!isClusterControl(codepoint) && !isWhitespaceCodepoint(codepoint)) {
			return false;
		}
	}
	return true;
}

static std::vector<GraphemeCluster> buildGraphemeClusters(std::u32string_view text) {
	auto breaks = graphemeBreaks(text);
	std::vector<GraphemeCluster> clusters;
	clusters.reserve(breaks.size());
	size_t start = 0;
	for (auto end : breaks) {
		clusters.push_back({ start, end, isWhitespaceCluster(text.substr(start, end - start)) });
		start = end;
	}
	return clusters;
}

struct TextCluster {
	explicit TextCluster(std::u32string_view text) :
			text(text), drawableText(drawableClusterText(text)), isWhitespace(isWhitespaceCluster(text)) {}

	std::u32string_view text;
	std::u32string drawableText;
	bool isWhitespace;
};

static bool fontFaceSupportsClusterWithoutShaping(const FontFace &face, const TextCluster &cluster) {
	if (face.isNull()) {
		return false;
	}
	for (auto codepoint : cluster.drawableText) {
		if (isWhitespaceCodepoint(codepoint)) {
			continue;
		}
		if (!fontFaceHasGlyph(face, codepoint)) {
			return false;
		}
	}
	return true;
}

struct ShapedRun {
	std::vector<SVGShapedGlyph> glyphs;
	std::vector<bool> supportedClusters;
	float width = 0;
};

#ifdef LUNASVG_ENABLE_HARFBUZZ
class HarfBuzzFont {
public:
	explicit HarfBuzzFont(const FontFace &fontFace) :
			m_fontFace(fontFace) {
		unsigned int dataLength = 0;
		int ttcIndex = 0;
		auto data = plutovg_font_face_get_data(m_fontFace.get(), &dataLength, &ttcIndex);
		if (data == nullptr || dataLength == 0) {
			return;
		}
		m_blob = hb_blob_create(static_cast<const char *>(data), dataLength, HB_MEMORY_MODE_READONLY, nullptr, nullptr);
		m_face = hb_face_create(m_blob, static_cast<unsigned int>(ttcIndex));
		m_upem = hb_face_get_upem(m_face);
		if (m_upem == 0) {
			return;
		}
		m_font = hb_font_create(m_face);
		hb_font_set_scale(m_font, static_cast<int>(m_upem), static_cast<int>(m_upem));
		hb_ot_font_set_funcs(m_font);
		m_buffer = hb_buffer_create();
	}

	~HarfBuzzFont() {
		if (m_buffer != nullptr) {
			hb_buffer_destroy(m_buffer);
		}
		if (m_font != nullptr) {
			hb_font_destroy(m_font);
		}
		if (m_face != nullptr) {
			hb_face_destroy(m_face);
		}
		if (m_blob != nullptr) {
			hb_blob_destroy(m_blob);
		}
	}

	HarfBuzzFont(const HarfBuzzFont &) = delete;
	HarfBuzzFont &operator=(const HarfBuzzFont &) = delete;
	HarfBuzzFont(HarfBuzzFont &&other) noexcept :
			m_fontFace(std::move(other.m_fontFace)),
			m_blob(std::exchange(other.m_blob, nullptr)),
			m_face(std::exchange(other.m_face, nullptr)),
			m_font(std::exchange(other.m_font, nullptr)),
			m_buffer(std::exchange(other.m_buffer, nullptr)),
			m_upem(std::exchange(other.m_upem, 0)) {}

	bool isValid() const { return m_font != nullptr && m_buffer != nullptr && m_upem != 0; }
	hb_font_t *font() const { return m_font; }
	hb_buffer_t *buffer() const { return m_buffer; }
	unsigned int upem() const { return m_upem; }

private:
	// Keep the backing bytes alive for as long as a cached hb_blob references
	// them. The LunaSVG face registry itself can be reset independently.
	FontFace m_fontFace;
	hb_blob_t *m_blob = nullptr;
	hb_face_t *m_face = nullptr;
	hb_font_t *m_font = nullptr;
	hb_buffer_t *m_buffer = nullptr;
	unsigned int m_upem = 0;
};

using HarfBuzzFontCache = std::unordered_map<const plutovg_font_face_t *, std::unique_ptr<HarfBuzzFont>>;

static HarfBuzzFontCache &harfBuzzFontCache();

static const HarfBuzzFont &harfBuzzFontForFace(const FontFace &face) {
	auto &cache = harfBuzzFontCache();
	auto key = face.get();
	auto it = cache.find(key);
	if (it == cache.end()) {
		it = cache.emplace(key, std::make_unique<HarfBuzzFont>(face)).first;
	}
	return *it->second;
}

static ShapedRun shapeRun(const HarfBuzzFont &font, float fontSize, std::u32string_view text,
		size_t start, size_t end, const std::vector<GraphemeCluster> &clusters,
		size_t firstCluster, size_t clusterCount, hb_direction_t direction, hb_script_t script, hb_language_t language) {
	ShapedRun result;
	if (!font.isValid()) {
		return result;
	}

	auto buffer = font.buffer();
	hb_buffer_reset(buffer);
	hb_buffer_set_cluster_level(buffer, HB_BUFFER_CLUSTER_LEVEL_MONOTONE_GRAPHEMES);
	hb_buffer_set_flags(buffer, HB_BUFFER_FLAG_REMOVE_DEFAULT_IGNORABLES);
	hb_buffer_add_utf32(buffer, reinterpret_cast<const uint32_t *>(text.data()), static_cast<int>(text.size()),
			static_cast<unsigned int>(start), static_cast<int>(end - start));
	// The run planner resolves these properties once for every fallback face.
	hb_buffer_set_direction(buffer, direction);
	hb_buffer_set_script(buffer, script);
	hb_buffer_set_language(buffer, language);
	hb_shape(font.font(), buffer, nullptr, 0);

	unsigned int glyphCount = 0;
	auto infos = hb_buffer_get_glyph_infos(buffer, &glyphCount);
	auto positions = hb_buffer_get_glyph_positions(buffer, &glyphCount);
	const auto scale = fontSize / static_cast<float>(font.upem());
	float penX = 0;
	float penY = 0;
	result.supportedClusters.assign(clusterCount, true);
	if (glyphCount == 0) {
		for (size_t i = 0; i < clusterCount; ++i) {
			result.supportedClusters[i] = clusters[firstCluster + i].isWhitespace;
		}
	}
	result.glyphs.reserve(glyphCount);
	for (unsigned int i = 0; i < glyphCount; ++i) {
		const auto clusterOffset = static_cast<size_t>(infos[i].cluster);
		if (infos[i].codepoint == 0) {
			for (size_t j = 0; j < clusterCount; ++j) {
				const auto &cluster = clusters[firstCluster + j];
				if (clusterOffset >= cluster.start && clusterOffset < cluster.end) {
					if (!cluster.isWhitespace) {
						result.supportedClusters[j] = false;
					}
					break;
				}
			}
		}
		result.glyphs.push_back({ infos[i].codepoint, penX + positions[i].x_offset * scale,
				penY + positions[i].y_offset * scale, clusterOffset });
		penX += positions[i].x_advance * scale;
		penY += positions[i].y_advance * scale;
	}
	if (penX < 0) {
		for (auto &glyph : result.glyphs) {
			glyph.x -= penX;
		}
	}
	result.width = std::abs(penX);
	if (shapingObserverFunction != nullptr) {
		std::vector<uint32_t> glyphIndices;
		std::vector<size_t> glyphClusters;
		std::vector<float> xPositions;
		std::vector<float> yPositions;
		glyphIndices.reserve(result.glyphs.size());
		glyphClusters.reserve(result.glyphs.size());
		xPositions.reserve(result.glyphs.size());
		yPositions.reserve(result.glyphs.size());
		for (const auto &glyph : result.glyphs) {
			glyphIndices.push_back(glyph.index);
			glyphClusters.push_back(glyph.cluster);
			xPositions.push_back(glyph.x);
			yPositions.push_back(glyph.y);
		}
		shapingObserverFunction(start, end, HB_DIRECTION_IS_BACKWARD(direction), static_cast<uint32_t>(script),
				glyphIndices.data(), glyphClusters.data(), xPositions.data(), yPositions.data(), glyphClusters.size(), result.width,
				shapingObserverClosure);
	}

	return result;
}

struct ShapingRunPlan {
	size_t firstCluster = 0;
	size_t clusterCount = 0;
	hb_script_t script = HB_SCRIPT_UNKNOWN;
	hb_direction_t direction = HB_DIRECTION_LTR;
	uint8_t bidiLevel = 0;
};

static bool isWeakScript(hb_script_t script) {
	return script == HB_SCRIPT_COMMON || script == HB_SCRIPT_INHERITED || script == HB_SCRIPT_UNKNOWN;
}

static std::vector<hb_script_t> resolvedClusterScripts(std::u32string_view text,
		const std::vector<GraphemeCluster> &clusters) {
	auto unicode = hb_unicode_funcs_get_default();
	std::vector<hb_script_t> scripts(clusters.size(), HB_SCRIPT_UNKNOWN);
	for (size_t i = 0; i < clusters.size(); ++i) {
		for (size_t offset = clusters[i].start; offset < clusters[i].end; ++offset) {
			auto script = hb_unicode_script(unicode, text[offset]);
			if (!isWeakScript(script)) {
				scripts[i] = script;
				break;
			}
		}
	}

	// Common and inherited clusters stay with an adjacent strong script. This
	// keeps punctuation and combining clusters in the same shaping context.
	hb_script_t previous = HB_SCRIPT_UNKNOWN;
	for (size_t i = 0; i < scripts.size(); ++i) {
		if (!isWeakScript(scripts[i])) {
			previous = scripts[i];
		} else if (!isWeakScript(previous)) {
			scripts[i] = previous;
		}
	}
	hb_script_t next = HB_SCRIPT_UNKNOWN;
	for (size_t i = scripts.size(); i-- > 0;) {
		if (!isWeakScript(scripts[i])) {
			next = scripts[i];
		} else if (!isWeakScript(next)) {
			scripts[i] = next;
		}
	}
	return scripts;
}

enum class ClusterDirection : uint8_t {
	Neutral,
	LeftToRight,
	RightToLeft,
	Number,
};

static bool isNumberCodepoint(uint32_t codepoint) {
	return (codepoint >= U'0' && codepoint <= U'9') || (codepoint >= 0x0660 && codepoint <= 0x0669) ||
			(codepoint >= 0x06F0 && codepoint <= 0x06F9);
}

static ClusterDirection directionForCluster(std::u32string_view text, const GraphemeCluster &cluster, hb_script_t script) {
	for (size_t offset = cluster.start; offset < cluster.end; ++offset) {
		if (isNumberCodepoint(text[offset])) {
			return ClusterDirection::Number;
		}
	}
	const auto scriptDirection = hb_script_get_horizontal_direction(script);
	if (scriptDirection == HB_DIRECTION_RTL) {
		return ClusterDirection::RightToLeft;
	}
	if (scriptDirection == HB_DIRECTION_LTR && !isWeakScript(script)) {
		return ClusterDirection::LeftToRight;
	}
	return ClusterDirection::Neutral;
}

static ClusterDirection resolvedNeutralDirection(const std::vector<ClusterDirection> &directions, size_t index,
		ClusterDirection baseDirection) {
	auto normalize = [](ClusterDirection direction) {
		return direction == ClusterDirection::Number ? ClusterDirection::LeftToRight : direction;
	};
	auto previous = baseDirection;
	for (size_t i = index; i-- > 0;) {
		if (directions[i] != ClusterDirection::Neutral) {
			previous = normalize(directions[i]);
			break;
		}
	}
	auto next = baseDirection;
	for (size_t i = index + 1; i < directions.size(); ++i) {
		if (directions[i] != ClusterDirection::Neutral) {
			next = normalize(directions[i]);
			break;
		}
	}
	return previous == next ? previous : baseDirection;
}

static std::vector<ShapingRunPlan> fallbackShapingRuns(std::u32string_view text,
		const std::vector<GraphemeCluster> &clusters, Direction paragraphDirection) {
	std::vector<ShapingRunPlan> runs;
	if (clusters.empty()) {
		return runs;
	}

	auto scripts = resolvedClusterScripts(text, clusters);

	const bool baseRightToLeft = paragraphDirection == Direction::Rtl;
	const auto baseDirection = baseRightToLeft ? ClusterDirection::RightToLeft : ClusterDirection::LeftToRight;
	std::vector<ClusterDirection> directions(clusters.size(), ClusterDirection::Neutral);
	for (size_t i = 0; i < clusters.size(); ++i) {
		directions[i] = directionForCluster(text, clusters[i], scripts[i]);
	}

	for (size_t i = 0; i < directions.size(); ++i) {
		if (directions[i] == ClusterDirection::Neutral) {
			directions[i] = resolvedNeutralDirection(directions, i, baseDirection);
		}
	}

	std::vector<uint8_t> levels(clusters.size(), baseRightToLeft ? 1 : 0);
	auto precedingStrong = baseDirection;
	for (size_t i = 0; i < directions.size(); ++i) {
		switch (directions[i]) {
			case ClusterDirection::RightToLeft:
				levels[i] = 1;
				precedingStrong = ClusterDirection::RightToLeft;
				break;
			case ClusterDirection::LeftToRight:
				levels[i] = baseRightToLeft ? 2 : 0;
				precedingStrong = ClusterDirection::LeftToRight;
				break;
			case ClusterDirection::Number:
				levels[i] = (baseRightToLeft || precedingStrong == ClusterDirection::RightToLeft) ? 2 : 0;
				break;
			case ClusterDirection::Neutral:
				break;
		}
	}

	size_t first = 0;
	for (size_t i = 1; i <= scripts.size(); ++i) {
		if (i == scripts.size() || scripts[i] != scripts[first] || levels[i] != levels[first]) {
			runs.push_back({ first, i - first, scripts[first],
					(levels[first] & 1) != 0 ? HB_DIRECTION_RTL : HB_DIRECTION_LTR, levels[first] });
			first = i;
		}
	}

	uint8_t maximumLevel = 0;
	uint8_t minimumOddLevel = UINT8_MAX;
	for (const auto &run : runs) {
		maximumLevel = std::max(maximumLevel, run.bidiLevel);
		if ((run.bidiLevel & 1) != 0) {
			minimumOddLevel = std::min(minimumOddLevel, run.bidiLevel);
		}
	}
	if (minimumOddLevel != UINT8_MAX) {
		for (int level = maximumLevel; level >= minimumOddLevel; --level) {
			size_t runStart = 0;
			while (runStart < runs.size()) {
				while (runStart < runs.size() && runs[runStart].bidiLevel < level) {
					++runStart;
				}
				size_t runEnd = runStart;
				while (runEnd < runs.size() && runs[runEnd].bidiLevel >= level) {
					++runEnd;
				}
				std::reverse(runs.begin() + runStart, runs.begin() + runEnd);
				runStart = runEnd;
			}
		}
	}
	return runs;
}

#ifdef LUNASVG_ENABLE_ICU_BIDI
static std::vector<ShapingRunPlan> shapingRuns(std::u32string_view text,
		const std::vector<GraphemeCluster> &clusters, Direction paragraphDirection) {
	if (clusters.empty()) {
		return {};
	}

	std::vector<UChar> utf16;
	utf16.reserve(text.size());
	std::vector<int32_t> utf16Offsets(text.size() + 1, 0);
	for (size_t i = 0; i < text.size(); ++i) {
		utf16Offsets[i] = static_cast<int32_t>(utf16.size());
		auto codepoint = static_cast<uint32_t>(text[i]);
		if (codepoint <= 0xFFFF) {
			utf16.push_back(static_cast<UChar>(codepoint));
		} else {
			codepoint -= 0x10000;
			utf16.push_back(static_cast<UChar>(0xD800 + (codepoint >> 10)));
			utf16.push_back(static_cast<UChar>(0xDC00 + (codepoint & 0x3FF)));
		}
	}
	utf16Offsets[text.size()] = static_cast<int32_t>(utf16.size());

	UErrorCode error = U_ZERO_ERROR;
	using UBiDiPtr = std::unique_ptr<UBiDi, decltype(&ubidi_close)>;
	UBiDiPtr bidi(ubidi_openSized(static_cast<int32_t>(utf16.size()), 0, &error), ubidi_close);
	if (U_FAILURE(error) || bidi == nullptr) {
		return fallbackShapingRuns(text, clusters, paragraphDirection);
	}
	ubidi_setPara(bidi.get(), utf16.data(), static_cast<int32_t>(utf16.size()),
			paragraphDirection == Direction::Rtl ? UBIDI_RTL : UBIDI_LTR, nullptr, &error);
	if (U_FAILURE(error)) {
		return fallbackShapingRuns(text, clusters, paragraphDirection);
	}

	const auto scripts = resolvedClusterScripts(text, clusters);
	const auto runCount = ubidi_countRuns(bidi.get(), &error);
	if (U_FAILURE(error)) {
		return fallbackShapingRuns(text, clusters, paragraphDirection);
	}

	std::vector<ShapingRunPlan> runs;
	for (int32_t runIndex = 0; runIndex < runCount; ++runIndex) {
		int32_t logicalStart16 = 0;
		int32_t logicalLength16 = 0;
		const auto bidiDirection = ubidi_getVisualRun(bidi.get(), runIndex, &logicalStart16, &logicalLength16);
		const auto logicalEnd16 = logicalStart16 + logicalLength16;
		auto startIt = std::lower_bound(utf16Offsets.begin(), utf16Offsets.end(), logicalStart16);
		auto endIt = std::lower_bound(utf16Offsets.begin(), utf16Offsets.end(), logicalEnd16);
		if (startIt == utf16Offsets.end() || endIt == utf16Offsets.end() || *startIt != logicalStart16 || *endIt != logicalEnd16) {
			return fallbackShapingRuns(text, clusters, paragraphDirection);
		}
		const auto logicalStart = static_cast<size_t>(startIt - utf16Offsets.begin());
		const auto logicalEnd = static_cast<size_t>(endIt - utf16Offsets.begin());

		size_t firstCluster = 0;
		while (firstCluster < clusters.size() && clusters[firstCluster].end <= logicalStart) {
			++firstCluster;
		}
		size_t endCluster = firstCluster;
		while (endCluster < clusters.size() && clusters[endCluster].start < logicalEnd) {
			++endCluster;
		}
		if (firstCluster == endCluster || clusters[firstCluster].start < logicalStart || clusters[endCluster - 1].end > logicalEnd) {
			return fallbackShapingRuns(text, clusters, paragraphDirection);
		}

		std::vector<ShapingRunPlan> scriptRuns;
		size_t scriptStart = firstCluster;
		for (size_t i = firstCluster + 1; i <= endCluster; ++i) {
			if (i == endCluster || scripts[i] != scripts[scriptStart]) {
				scriptRuns.push_back({ scriptStart, i - scriptStart, scripts[scriptStart],
						bidiDirection == UBIDI_RTL ? HB_DIRECTION_RTL : HB_DIRECTION_LTR, 0 });
				scriptStart = i;
			}
		}
		if (bidiDirection == UBIDI_RTL) {
			std::reverse(scriptRuns.begin(), scriptRuns.end());
		}
		runs.insert(runs.end(), scriptRuns.begin(), scriptRuns.end());
	}
	return runs;
}
#else
static std::vector<ShapingRunPlan> shapingRuns(std::u32string_view text,
		const std::vector<GraphemeCluster> &clusters, Direction paragraphDirection) {
	return fallbackShapingRuns(text, clusters, paragraphDirection);
}
#endif
#endif

struct ThreadTextCaches {
	EmbeddedSVGGlyphBitmapCache embeddedSVGGlyphBitmaps;
#ifdef LUNASVG_ENABLE_HARFBUZZ
	HarfBuzzFontCache harfBuzzFonts;
#endif

	void clear() {
		embeddedSVGGlyphBitmaps.clear();
#ifdef LUNASVG_ENABLE_HARFBUZZ
		harfBuzzFonts.clear();
#endif
	}
};

static ThreadTextCaches &threadTextCaches() {
	static thread_local ThreadTextCaches caches;
	return caches;
}

static EmbeddedSVGGlyphBitmapCache &embeddedSVGGlyphBitmapCache() {
	return threadTextCaches().embeddedSVGGlyphBitmaps;
}

#ifdef LUNASVG_ENABLE_HARFBUZZ
static HarfBuzzFontCache &harfBuzzFontCache() {
	return threadTextCaches().harfBuzzFonts;
}
#endif

struct FontCandidate {
	explicit FontCandidate(FontFace fontFace) :
			face(std::move(fontFace))
#ifdef LUNASVG_ENABLE_HARFBUZZ
			,
			shapingFont(&harfBuzzFontForFace(face))
#endif
	{
	}

	FontFace face;
#ifdef LUNASVG_ENABLE_HARFBUZZ
	const HarfBuzzFont *shapingFont = nullptr;
#endif
};

#ifdef LUNASVG_ENABLE_HARFBUZZ
struct SelectedFontGroup {
	size_t begin = 0;
	size_t end = 0;
	int candidateIndex = -1;
};

struct ShapedFontSpan {
	size_t start = 0;
	size_t end = 0;
	int candidateIndex = -1;
	std::vector<SVGShapedGlyph> glyphs;
	float width = 0;
	bool missing = false;
	bool whitespace = false;
};

static std::vector<int> selectFallbackFonts(const std::vector<FontCandidate> &candidates, float fontSize,
		std::u32string_view text, const std::vector<GraphemeCluster> &clusters, const ShapingRunPlan &run,
		hb_language_t language) {
	const auto runEndCluster = run.firstCluster + run.clusterCount;
	const auto runStart = clusters[run.firstCluster].start;
	const auto runEnd = clusters[runEndCluster - 1].end;
	std::vector<int> selected(run.clusterCount, -1);
	for (size_t candidateIndex = 0; candidateIndex < candidates.size(); ++candidateIndex) {
		auto probe = shapeRun(*candidates[candidateIndex].shapingFont, fontSize, text, runStart, runEnd,
				clusters, run.firstCluster, run.clusterCount, run.direction, run.script, language);
		if (probe.supportedClusters.size() != run.clusterCount) {
			continue;
		}
		bool allSelected = true;
		for (size_t i = 0; i < run.clusterCount; ++i) {
			if (selected[i] < 0 && probe.supportedClusters[i]) {
				selected[i] = static_cast<int>(candidateIndex);
			}
			allSelected = allSelected && selected[i] >= 0;
		}
		if (allSelected) {
			break;
		}
	}
	return selected;
}

static std::vector<SelectedFontGroup> groupSelectedFonts(const std::vector<int> &selected,
		const ShapingRunPlan &run, const std::vector<GraphemeCluster> &clusters, size_t textStartOffset,
		const SVGCharacterPositions &characterPositions) {
	std::vector<SelectedFontGroup> groups;
	size_t group = 0;
	while (group < run.clusterCount) {
		const auto candidateIndex = selected[group];
		size_t groupEnd = group + 1;
		while (groupEnd < run.clusterCount && selected[groupEnd] == candidateIndex) {
			const auto absoluteStart = textStartOffset + clusters[run.firstCluster + groupEnd].start;
			// Per-character SVG positioning is itself a shaping boundary.
			if (characterPositions.find(absoluteStart) != characterPositions.end()) {
				break;
			}
			++groupEnd;
		}
		groups.push_back({ group, groupEnd, candidateIndex });
		group = groupEnd;
	}
	return groups;
}

static std::vector<ShapedFontSpan> shapeSelectedGroups(const std::vector<SelectedFontGroup> &groups,
		const std::vector<FontCandidate> &candidates, float fontSize, std::u32string_view text,
		const std::vector<GraphemeCluster> &clusters, const ShapingRunPlan &run, hb_language_t language) {
	std::vector<ShapedFontSpan> spans;
	spans.reserve(groups.size());
	auto shapeGroup = [&](const SelectedFontGroup &group) {
		const auto firstCluster = run.firstCluster + group.begin;
		const auto clusterCount = group.end - group.begin;
		const auto start = clusters[firstCluster].start;
		const auto end = clusters[firstCluster + clusterCount - 1].end;
		bool whitespace = true;
		for (size_t i = 0; i < clusterCount; ++i) {
			whitespace = whitespace && clusters[firstCluster + i].isWhitespace;
		}

		if (group.candidateIndex >= 0) {
			auto shaped = shapeRun(*candidates[group.candidateIndex].shapingFont, fontSize, text, start, end,
					clusters, firstCluster, clusterCount, run.direction, run.script, language);
			spans.push_back({ start, end, group.candidateIndex, std::move(shaped.glyphs), shaped.width, false, whitespace });
			return;
		}

		// Missing-glyph boxes remain one per grapheme cluster and follow the
		// visual direction of their shaping run.
		for (size_t visualIndex = 0; visualIndex < clusterCount; ++visualIndex) {
			const auto logicalIndex = run.direction == HB_DIRECTION_RTL ? clusterCount - visualIndex - 1 : visualIndex;
			const auto &cluster = clusters[firstCluster + logicalIndex];
			spans.push_back({ cluster.start, cluster.end, -1, {}, 0, !cluster.isWhitespace, cluster.isWhitespace });
		}
	};

	if (run.direction == HB_DIRECTION_RTL) {
		for (auto it = groups.rbegin(); it != groups.rend(); ++it) {
			shapeGroup(*it);
		}
	} else {
		for (const auto &group : groups) {
			shapeGroup(group);
		}
	}
	return spans;
}
#endif

static std::vector<FontCandidate> resolveFontCandidates(const SVGTextPositioningElement *element) {
	FontFamilyList families;
	if (!element->font_family().empty()) {
		families = parseFontFamilyList(element->font_family());
	} else if (fontPreferencesConfigured()) {
		families = fontPreferences();
	}

	std::vector<FontCandidate> candidates;
	for (const auto &family : families) {
		auto face = fontFaceCache()->getFontFace(family, element->font_bold(), element->font_italic());
		if (!face.isNull()) {
			candidates.emplace_back(std::move(face));
		}
	}
	if (candidates.empty() && !fontPreferencesConfigured() && !element->font().isNull()) {
		candidates.emplace_back(element->font().face());
	}
	return candidates;
}

static bool tryResolveEmbeddedSVGGlyph(const Font &font, uint32_t glyphIndex, const Point &origin, EmbeddedSVGGlyphInfo &glyph) {
	auto face = font.face().get();
	if (face == nullptr) {
		return false;
	}

	auto svgLength = plutovg_font_face_get_glyph_index_svg(face, glyphIndex, &glyph.svgData);
	if (svgLength <= 0 || glyph.svgData == nullptr) {
		return false;
	}

	plutovg_rect_t glyphExtents = { 0 };
	plutovg_font_face_get_glyph_index_metrics(face, font.size(), glyphIndex, nullptr, nullptr, &glyphExtents);
	glyph.svgLength = static_cast<size_t>(svgLength);
	glyph.dstRect = Rect(origin.x + glyphExtents.x, origin.y + glyphExtents.y, glyphExtents.w, glyphExtents.h);
	return !glyph.dstRect.isEmpty();
}

static const Bitmap *getCachedEmbeddedSVGGlyphBitmap(const Font &font, uint32_t glyphIndex, std::string_view svgDocument) {
	auto face = font.face().get();
	if (face == nullptr || svgDocument.empty()) {
		return nullptr;
	}

	// Cache rasterized embedded SVG glyphs per thread to avoid reparsing the
	// embedded SVG document on every frame.
	auto &bitmapCache = embeddedSVGGlyphBitmapCache();
	EmbeddedSVGGlyphBitmapCacheKey key{ face, glyphIndex };
	auto it = bitmapCache.find(key);
	if (it != bitmapCache.end()) {
		return it->second.isNull() ? nullptr : &it->second;
	}

	auto document = Document::loadFromData(svgDocument.data(), svgDocument.size());
	if (!document) {
		bitmapCache.emplace(key, Bitmap());
		return nullptr;
	}
	auto bounds = Rect(document->boundingBox());
	if (bounds.isEmpty()) {
		bitmapCache.emplace(key, Bitmap());
		return nullptr;
	}

	auto bitmapWidth = std::max(1, static_cast<int>(std::ceil(bounds.w)));
	auto bitmapHeight = std::max(1, static_cast<int>(std::ceil(bounds.h)));
	Bitmap bitmap(bitmapWidth, bitmapHeight);
	bitmap.clear(0x00000000);
	document->render(bitmap, Matrix::translated(-bounds.x, -bounds.y));
	return &bitmapCache.emplace(key, std::move(bitmap)).first->second;
}

static bool tryRenderEmbeddedSVGGlyph(const SVGTextFragment &fragment, const SVGShapedGlyph &shapedGlyph,
		const Transform &transform, SVGRenderState &state) {
	const auto &font = fragment.font;
	auto origin = Point(fragment.x + shapedGlyph.x, fragment.y - shapedGlyph.y);
	EmbeddedSVGGlyphInfo glyph;
	if (!tryResolveEmbeddedSVGGlyph(font, shapedGlyph.index, origin, glyph)) {
		return false;
	}

	auto bitmap = getCachedEmbeddedSVGGlyphBitmap(font, shapedGlyph.index, std::string_view(glyph.svgData, glyph.svgLength));
	if (bitmap == nullptr) {
		return false;
	}

	state->drawImage(*bitmap, glyph.dstRect, Rect(0, 0, bitmap->width(), bitmap->height()), transform);
	return true;
}

static Path shapedGlyphPath(const SVGTextFragment &fragment, const SVGShapedGlyph &glyph) {
	Path path;
	auto face = fragment.font.face().get();
	if (face == nullptr) {
		return path;
	}
	path.addGlyph(face, fragment.font.size(), fragment.x + glyph.x, fragment.y - glyph.y, glyph.index);
	return path;
}

static Path shapedGlyphPath(const SVGTextFragment &fragment) {
	Path path;
	auto face = fragment.font.face().get();
	if (face == nullptr) {
		return path;
	}
	for (const auto &glyph : fragment.glyphs) {
		path.addGlyph(face, fragment.font.size(), fragment.x + glyph.x, fragment.y - glyph.y, glyph.index);
	}
	return path;
}

static Rect shapedGlyphBounds(const SVGTextFragment &fragment) {
	auto bounds = Rect::Invalid;
	auto face = fragment.font.face().get();
	if (face == nullptr) {
		return Rect::Empty;
	}
	for (const auto &glyph : fragment.glyphs) {
		EmbeddedSVGGlyphInfo embeddedGlyph;
		auto origin = Point(fragment.x + glyph.x, fragment.y - glyph.y);
		if (tryResolveEmbeddedSVGGlyph(fragment.font, glyph.index, origin, embeddedGlyph)) {
			bounds.unite(embeddedGlyph.dstRect);
			continue;
		}
		plutovg_rect_t extents = { 0 };
		plutovg_font_face_get_glyph_index_metrics(face, fragment.font.size(), glyph.index, nullptr, nullptr, &extents);
		bounds.unite(Rect(fragment.x + glyph.x + extents.x, fragment.y - glyph.y + extents.y, extents.w, extents.h));
	}
	return bounds.isValid() ? bounds : Rect::Empty;
}

static Rect missingGlyphRect(const SVGTextFragment &fragment) {
	const auto fontSize = fragment.element->font_size();
	const auto inset = std::max(0.5f, fontSize * 0.05f);
	return Rect(fragment.x + inset, fragment.y - fontSize * 0.8f, std::max(1.f, fragment.width - inset * 2.f),
			std::max(1.f, fontSize * 0.8f));
}

static Path missingGlyphPath(const SVGTextFragment &fragment) {
	auto outer = missingGlyphRect(fragment);
	auto inner = outer;
	inner.inflate(-std::max(1.f, fragment.element->font_size() * 0.08f));
	Path path;
	path.addRect(outer);
	if (!inner.isEmpty()) {
		path.addRect(inner);
	}
	return path;
}

} // namespace

void clearTextCaches() {
	threadTextCaches().clear();
}

inline const SVGTextNode *toSVGTextNode(const SVGNode *node) {
	assert(node && node->isTextNode());
	return static_cast<const SVGTextNode *>(node);
}

inline const SVGTextPositioningElement *toSVGTextPositioningElement(const SVGNode *node) {
	assert(node && node->isTextPositioningElement());
	return static_cast<const SVGTextPositioningElement *>(node);
}

static AlignmentBaseline resolveDominantBaseline(const SVGTextPositioningElement *element) {
	switch (element->dominant_baseline()) {
		case DominantBaseline::Auto:
		case DominantBaseline::UseScript:
		case DominantBaseline::NoChange:
		case DominantBaseline::ResetSize:
			return AlignmentBaseline::Auto;
		case DominantBaseline::Ideographic:
			return AlignmentBaseline::Ideographic;
		case DominantBaseline::Alphabetic:
			return AlignmentBaseline::Alphabetic;
		case DominantBaseline::Hanging:
			return AlignmentBaseline::Hanging;
		case DominantBaseline::Mathematical:
			return AlignmentBaseline::Mathematical;
		case DominantBaseline::Central:
			return AlignmentBaseline::Central;
		case DominantBaseline::Middle:
			return AlignmentBaseline::Middle;
		case DominantBaseline::TextAfterEdge:
			return AlignmentBaseline::TextAfterEdge;
		case DominantBaseline::TextBeforeEdge:
			return AlignmentBaseline::TextBeforeEdge;
		default:
			assert(false);
	}

	return AlignmentBaseline::Auto;
}

static float calculateBaselineOffset(const SVGTextPositioningElement *element) {
	auto offset = element->baseline_offset();
	for (auto parent = element->parent(); parent->isTextPositioningElement(); parent = parent->parent()) {
		offset += toSVGTextPositioningElement(parent)->baseline_offset();
	}

	auto baseline = element->alignment_baseline();
	if (baseline == AlignmentBaseline::Auto || baseline == AlignmentBaseline::Baseline) {
		baseline = resolveDominantBaseline(element);
	}

	const auto &font = element->font();
	switch (baseline) {
		case AlignmentBaseline::BeforeEdge:
		case AlignmentBaseline::TextBeforeEdge:
			offset -= font.ascent();
			break;
		case AlignmentBaseline::Middle:
			offset -= font.xHeight() / 2.f;
			break;
		case AlignmentBaseline::Central:
			offset -= (font.ascent() + font.descent()) / 2.f;
			break;
		case AlignmentBaseline::AfterEdge:
		case AlignmentBaseline::TextAfterEdge:
		case AlignmentBaseline::Ideographic:
			offset -= font.descent();
			break;
		case AlignmentBaseline::Hanging:
			offset -= font.ascent() * 8.f / 10.f;
			break;
		case AlignmentBaseline::Mathematical:
			offset -= font.ascent() / 2.f;
			break;
		default:
			break;
	}

	return offset;
}

static bool needsTextAnchorAdjustment(const SVGTextPositioningElement *element) {
	auto direction = element->direction();
	switch (element->text_anchor()) {
		case TextAnchor::Start:
			return direction == Direction::Rtl;
		case TextAnchor::Middle:
			return true;
		case TextAnchor::End:
			return direction == Direction::Ltr;
		default:
			assert(false);
	}

	return false;
}

static float calculateTextAnchorOffset(const SVGTextPositioningElement *element, float width) {
	auto direction = element->direction();
	switch (element->text_anchor()) {
		case TextAnchor::Start:
			if (direction == Direction::Ltr) {
				return 0.f;
			}
			return -width;
		case TextAnchor::Middle:
			return -width / 2.f;
		case TextAnchor::End:
			if (direction == Direction::Ltr) {
				return -width;
			}
			return 0.f;
		default:
			assert(false);
	}

	return 0.f;
}

static void adjustTextAnchor(SVGTextFragmentList::iterator begin, SVGTextFragmentList::iterator end) {
	if (!needsTextAnchorAdjustment(begin->element)) {
		return;
	}
	float chunkWidth = 0.f;
	const SVGTextFragment *lastFragment = nullptr;
	for (auto it = begin; it != end; ++it) {
		const SVGTextFragment &fragment = *it;
		chunkWidth += fragment.width;
		if (lastFragment) {
			chunkWidth += fragment.x - (lastFragment->x + lastFragment->width);
		}
		lastFragment = &fragment;
	}

	auto chunkOffset = calculateTextAnchorOffset(begin->element, chunkWidth);
	for (auto it = begin; it != end; ++it) {
		SVGTextFragment &fragment = *it;
		fragment.x += chunkOffset;
	}
}

SVGTextFragmentsBuilder::SVGTextFragmentsBuilder(std::u32string &text, SVGTextFragmentList &fragments) :
		m_text(text), m_fragments(fragments) {
	m_text.clear();
	m_fragments.clear();
}

void SVGTextFragmentsBuilder::build(const SVGTextElement *textElement) {
	handleElement(textElement);
	for (const auto &position : m_textPositions) {
		fillCharacterPositions(position);
	}

	for (const auto &position : m_textPositions) {
		buildTextNodeFragments(position);
	}
	adjustTextAnchors();
}

void SVGTextFragmentsBuilder::appendFragment(const SVGTextPosition &textPosition,
		const SVGTextPositioningElement *element, float baselineOffset, size_t localStart, size_t localEnd,
		Font font, std::vector<SVGShapedGlyph> glyphs, float width, bool missing, bool whitespace) {
	const auto startOffset = textPosition.startOffset + localStart;
	const auto endOffset = textPosition.startOffset + localEnd;
	SVGCharacterPosition characterPosition;
	auto positionIt = m_characterPositions.find(startOffset);
	if (positionIt != m_characterPositions.end()) {
		characterPosition = positionIt->second;
	}

	auto angle = characterPosition.rotate.value_or(0);
	auto dx = characterPosition.dx.value_or(0);
	auto dy = characterPosition.dy.value_or(0);
	m_x = dx + characterPosition.x.value_or(m_x);
	m_y = dy + characterPosition.y.value_or(m_y);

	SVGTextFragment fragment(element);
	fragment.offset = startOffset;
	fragment.length = endOffset - startOffset;
	fragment.font = std::move(font);
	for (auto &glyph : glyphs) {
		glyph.cluster += textPosition.startOffset;
	}
	fragment.glyphs = std::move(glyphs);
	fragment.x = m_x;
	fragment.y = m_y - baselineOffset;
	fragment.angle = angle;
	fragment.startsNewTextChunk =
			(characterPosition.x || characterPosition.y) && startOffset == textPosition.startOffset;
	fragment.isMissingGlyph = missing;
	fragment.isWhitespace = whitespace;
	if (fragment.isMissingGlyph) {
		fragment.width = std::max(1.f, element->font_size() * 0.6f);
	} else if (fragment.font.isNull()) {
		fragment.width = fragment.isWhitespace ? std::max(1.f, element->font_size() * 0.33f) : 0.f;
	} else {
		fragment.width = width;
	}
	m_fragments.push_back(std::move(fragment));
	m_x += m_fragments.back().width;
}

void SVGTextFragmentsBuilder::buildTextNodeFragments(const SVGTextPosition &textPosition) {
	if (!textPosition.node->isTextNode()) {
		return;
	}

	std::u32string_view wholeText(m_text);
	auto element = toSVGTextPositioningElement(textPosition.node->parent());
	const auto candidates = resolveFontCandidates(element);
	auto baselineOffset = calculateBaselineOffset(element);
	auto nodeText = wholeText.substr(textPosition.startOffset, textPosition.endOffset - textPosition.startOffset);
	auto clusters = buildGraphemeClusters(nodeText);

#ifdef LUNASVG_ENABLE_HARFBUZZ
	const auto language = hb_language_from_string(element->language().c_str(), -1);
	const auto fontSize = element->font().size();
	for (const auto &shapingRun : shapingRuns(nodeText, clusters, element->direction())) {
		// Shape the complete bidi/script run with every candidate only to decide
		// fallback at grapheme granularity. A .notdef marks the owning grapheme
		// cluster as unsupported; successful neighboring clusters keep context.
		auto selected = selectFallbackFonts(candidates, fontSize, nodeText, clusters, shapingRun, language);
		auto groups = groupSelectedFonts(selected, shapingRun, clusters, textPosition.startOffset, m_characterPositions);
		auto spans = shapeSelectedGroups(groups, candidates, fontSize, nodeText, clusters, shapingRun, language);
		for (auto &span : spans) {
			Font font = span.candidateIndex >= 0 ? Font(candidates[span.candidateIndex].face, fontSize) : (candidates.empty() ? Font(FontFace(), fontSize) : Font(candidates.front().face, fontSize));
			appendFragment(textPosition, element, baselineOffset, span.start, span.end, std::move(font),
					std::move(span.glyphs), span.width, span.missing, span.whitespace);
		}
	}
#else
	for (const auto &clusterRange : clusters) {
		const TextCluster cluster(nodeText.substr(clusterRange.start, clusterRange.end - clusterRange.start));
		bool selected = false;
		for (const auto &candidate : candidates) {
			if (fontFaceSupportsClusterWithoutShaping(candidate.face, cluster)) {
				auto font = Font(candidate.face, element->font().size());
				auto width = font.measureText(cluster.drawableText);
				appendFragment(textPosition, element, baselineOffset, clusterRange.start, clusterRange.end, std::move(font), {}, width, false,
						cluster.isWhitespace);
				selected = true;
				break;
			}
		}
		if (!selected) {
			auto fontSize = element->font().size();
			Font missingFont = candidates.empty() ? Font(FontFace(), fontSize) : Font(candidates.front().face, fontSize);
			appendFragment(textPosition, element, baselineOffset, clusterRange.start, clusterRange.end, std::move(missingFont), {}, 0,
					!cluster.isWhitespace, cluster.isWhitespace);
		}
	}
#endif
}

void SVGTextFragmentsBuilder::adjustTextAnchors() {
	if (m_fragments.empty()) {
		return;
	}
	auto it = m_fragments.begin();
	auto begin = m_fragments.begin();
	auto end = m_fragments.end();
	for (++it; it != end; ++it) {
		const SVGTextFragment &fragment = *it;
		if (!fragment.startsNewTextChunk) {
			continue;
		}
		adjustTextAnchor(begin, it);
		begin = it;
	}

	adjustTextAnchor(begin, it);
}

void SVGTextFragmentsBuilder::handleText(const SVGTextNode *node) {
	const auto &text = node->data();
	if (text.empty()) {
		return;
	}
	auto element = toSVGTextPositioningElement(node->parent());
	const auto startOffset = m_text.length();
	uint32_t lastCharacter = ' ';
	if (!m_text.empty()) {
		lastCharacter = m_text.back();
	}

	plutovg_text_iterator_t it;
	plutovg_text_iterator_init(&it, text.data(), text.length(), PLUTOVG_TEXT_ENCODING_UTF8);
	while (plutovg_text_iterator_has_next(&it)) {
		auto currentCharacter = plutovg_text_iterator_next(&it);
		if (currentCharacter == '\t' || currentCharacter == '\n' || currentCharacter == '\r') {
			currentCharacter = ' ';
		}
		if (currentCharacter == ' ' && lastCharacter == ' ' && element->white_space() == WhiteSpace::Default) {
			continue;
		}
		m_text.push_back(currentCharacter);
		lastCharacter = currentCharacter;
	}

	if (startOffset < m_text.length()) {
		m_textPositions.emplace_back(node, startOffset, m_text.length());
	}
}

void SVGTextFragmentsBuilder::handleElement(const SVGTextPositioningElement *element) {
	auto itemIndex = m_textPositions.size();
	m_textPositions.emplace_back(element, m_text.length(), m_text.length());
	for (const auto &child : element->children()) {
		if (child->isTextNode()) {
			handleText(toSVGTextNode(child.get()));
		} else if (child->isTextPositioningElement()) {
			handleElement(toSVGTextPositioningElement(child.get()));
		}
	}

	auto &position = m_textPositions[itemIndex];
	assert(position.node == element);
	position.endOffset = m_text.length();
}

void SVGTextFragmentsBuilder::fillCharacterPositions(const SVGTextPosition &position) {
	if (!position.node->isTextPositioningElement()) {
		return;
	}
	auto element = toSVGTextPositioningElement(position.node);
	const auto &xList = element->x();
	const auto &yList = element->y();
	const auto &dxList = element->dx();
	const auto &dyList = element->dy();
	const auto &rotateList = element->rotate();

	auto xListSize = xList.size();
	auto yListSize = yList.size();
	auto dxListSize = dxList.size();
	auto dyListSize = dyList.size();
	auto rotateListSize = rotateList.size();
	if (!xListSize && !yListSize && !dxListSize && !dyListSize && !rotateListSize) {
		return;
	}

	LengthContext lengthContext(element);
	std::optional<float> lastRotation;
	for (auto offset = position.startOffset; offset < position.endOffset; ++offset) {
		auto index = offset - position.startOffset;
		if (index >= xListSize && index >= yListSize && index >= dxListSize && index >= dyListSize && index >= rotateListSize) {
			break;
		}
		auto &characterPosition = m_characterPositions[offset];
		if (index < xListSize) {
			characterPosition.x = lengthContext.valueForLength(xList[index], LengthDirection::Horizontal);
		}
		if (index < yListSize) {
			characterPosition.y = lengthContext.valueForLength(yList[index], LengthDirection::Vertical);
		}
		if (index < dxListSize) {
			characterPosition.dx = lengthContext.valueForLength(dxList[index], LengthDirection::Horizontal);
		}
		if (index < dyListSize) {
			characterPosition.dy = lengthContext.valueForLength(dyList[index], LengthDirection::Vertical);
		}
		if (index < rotateListSize) {
			characterPosition.rotate = rotateList[index];
			lastRotation = characterPosition.rotate;
		}
	}

	if (lastRotation == std::nullopt) {
		return;
	}
	auto offset = position.startOffset + rotateList.size();
	while (offset < position.endOffset) {
		m_characterPositions[offset++].rotate = lastRotation;
	}
}

SVGTextPositioningElement::SVGTextPositioningElement(Document *document, ElementID id) :
		SVGGraphicsElement(document, id), m_x(PropertyID::X, LengthDirection::Horizontal, LengthNegativeMode::Allow), m_y(PropertyID::Y, LengthDirection::Vertical, LengthNegativeMode::Allow), m_dx(PropertyID::Dx, LengthDirection::Horizontal, LengthNegativeMode::Allow), m_dy(PropertyID::Dy, LengthDirection::Vertical, LengthNegativeMode::Allow), m_rotate(PropertyID::Rotate) {
	addProperty(m_x);
	addProperty(m_y);
	addProperty(m_dx);
	addProperty(m_dy);
	addProperty(m_rotate);
}

void SVGTextPositioningElement::layoutElement(const SVGLayoutState &state) {
	m_font = state.font();
	m_font_family = state.font_family();
	m_language = "und";
	for (const SVGElement *element = this; element != nullptr; element = element->parent()) {
		if (element->hasAttribute(PropertyID::Lang) && !element->getAttribute(PropertyID::Lang).empty()) {
			m_language = element->getAttribute(PropertyID::Lang);
			break;
		}
	}
	m_font_bold = state.font_weight() == FontWeight::Bold;
	m_font_italic = state.font_style() == FontStyle::Italic;
	m_fill = getPaintServer(state.fill(), state.fill_opacity());
	m_stroke = getPaintServer(state.stroke(), state.stroke_opacity());
	SVGGraphicsElement::layoutElement(state);

	LengthContext lengthContext(this);
	m_stroke_width = lengthContext.valueForLength(state.stroke_width(), LengthDirection::Diagonal);
	m_baseline_offset = convertBaselineOffset(state.baseline_shit());
	m_alignment_baseline = state.alignment_baseline();
	m_dominant_baseline = state.dominant_baseline();
	m_text_anchor = state.text_anchor();
	m_white_space = state.white_space();
	m_direction = state.direction();
}

float SVGTextPositioningElement::convertBaselineOffset(const BaselineShift &baselineShift) const {
	if (baselineShift.type() == BaselineShift::Type::Baseline) {
		return 0.f;
	}
	if (baselineShift.type() == BaselineShift::Type::Sub) {
		return -m_font.height() / 2.f;
	}
	if (baselineShift.type() == BaselineShift::Type::Super) {
		return m_font.height() / 2.f;
	}

	const auto &length = baselineShift.length();
	if (length.units() == LengthUnits::Percent) {
		return length.value() * m_font.size() / 100.f;
	}
	if (length.units() == LengthUnits::Ex) {
		return length.value() * m_font.size() / 2.f;
	}
	if (length.units() == LengthUnits::Em) {
		return length.value() * m_font.size();
	}
	return length.value();
}

SVGTSpanElement::SVGTSpanElement(Document *document) :
		SVGTextPositioningElement(document, ElementID::Tspan) {
}

SVGTextElement::SVGTextElement(Document *document) :
		SVGTextPositioningElement(document, ElementID::Text) {
}

void SVGTextElement::layout(SVGLayoutState &state) {
	SVGTextPositioningElement::layout(state);
	SVGTextFragmentsBuilder fragmentsBuilder(m_text, m_fragments);
	fragmentsBuilder.build(this);
}

void SVGTextElement::render(SVGRenderState &state) const {
	if (m_text.empty() || isVisibilityHidden() || isDisplayNone()) {
		return;
	}
	SVGBlendInfo blendInfo(this);
	SVGRenderState newState(this, state, localTransform());
	newState.beginGroup(blendInfo);
	if (newState.mode() == SVGRenderMode::Clipping) {
		newState->setColor(Color::White);
	}

	std::u32string_view wholeText(m_text);
	for (const auto &fragment : m_fragments) {
		auto transform = newState.currentTransform() * Transform::rotated(fragment.angle, fragment.x, fragment.y);
		if (fragment.isWhitespace) {
			continue;
		}
		auto text = fragment.glyphs.empty() && !fragment.isMissingGlyph
				? drawableClusterText(wholeText.substr(fragment.offset, fragment.length))
				: std::u32string();

		if (newState.mode() == SVGRenderMode::Clipping) {
			if (fragment.isMissingGlyph) {
				newState->fillPath(missingGlyphPath(fragment), FillRule::EvenOdd, transform);
			} else if (!fragment.glyphs.empty()) {
				newState->fillPath(shapedGlyphPath(fragment), FillRule::NonZero, transform);
			} else {
				newState->fillText(text, fragment.font, Point(fragment.x, fragment.y), transform);
			}
		} else {
			const auto &fill = fragment.element->fill();
			const auto &stroke = fragment.element->stroke();
			auto stroke_width = fragment.element->stroke_width();
			if (fragment.isMissingGlyph) {
				auto path = missingGlyphPath(fragment);
				if (fill.applyPaint(newState)) {
					newState->fillPath(path, FillRule::EvenOdd, transform);
				}
				if (stroke.applyPaint(newState)) {
					newState->strokePath(path, StrokeData(stroke_width), transform);
				}
				continue;
			}
			if (fragment.glyphs.empty()) {
				if (fill.applyPaint(newState)) {
					newState->fillText(text, fragment.font, Point(fragment.x, fragment.y), transform);
				}
				if (stroke.applyPaint(newState)) {
					newState->strokeText(text, stroke_width, fragment.font, Point(fragment.x, fragment.y), transform);
				}
				continue;
			}

			// A shaped font run can mix outline glyphs and multiple SVG-in-OT
			// glyphs. Decide per glyph so fallback-run coalescing never strips
			// color from adjacent emoji.
			for (const auto &glyph : fragment.glyphs) {
				if (tryRenderEmbeddedSVGGlyph(fragment, glyph, transform, newState)) {
					continue;
				}
				auto path = shapedGlyphPath(fragment, glyph);
				if (fill.applyPaint(newState)) {
					newState->fillPath(path, FillRule::NonZero, transform);
				}
				if (stroke.applyPaint(newState)) {
					newState->strokePath(path, StrokeData(stroke_width), transform);
				}
			}
		}
	}

	newState.endGroup(blendInfo);
}

Rect SVGTextElement::boundingBox(bool includeStroke) const {
	auto boundingBox = Rect::Invalid;
	for (const auto &fragment : m_fragments) {
		if (fragment.isWhitespace) {
			continue;
		}
		const auto &font = fragment.font;
		const auto &stroke = fragment.element->stroke();
		auto fragmentTranform = Transform::rotated(fragment.angle, fragment.x, fragment.y);
		auto fragmentRect = fragment.isMissingGlyph ? missingGlyphRect(fragment)
													: (fragment.glyphs.empty()
																	  ? Rect(fragment.x, fragment.y - font.ascent(), fragment.width,
																				fragment.element->font_size())
																	  : shapedGlyphBounds(fragment));
		if (includeStroke && stroke.isRenderable()) {
			fragmentRect.inflate(fragment.element->stroke_width() / 2.f);
		}
		boundingBox.unite(fragmentTranform.mapRect(fragmentRect));
	}

	if (!boundingBox.isValid()) {
		boundingBox = Rect::Empty;
	}
	return boundingBox;
}

} // namespace lunasvg
