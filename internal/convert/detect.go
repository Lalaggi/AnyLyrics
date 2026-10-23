package convert

import "strings"

// Detect guesses the format of pre-split lines per LRC+ v3 Annex C.
func Detect(lines []string) Format {
	// 1. Timing-shaped lines with timestamp (or v1-bulk) units → LRC+.
	for _, s := range lines {
		if timingShaped(s) && isLRCPlusUnits(s) {
			return FormatLRCPlus
		}
	}
	// XML prolog or root element → TTML.
	for _, s := range lines {
		if s == "" {
			continue
		}
		if looksLikeXML(s) {
			return FormatTTML
		}
		break
	}
	// 2. Headers with inline word tags → ELRC. 3. Any header → LRC.
	sawHeader := false
	for _, s := range lines {
		if _, text, ok := parseHeader(s); ok {
			sawHeader = true
			if hasInlineWordTag(text) {
				return FormatELRC
			}
		}
	}
	if sawHeader {
		return FormatLRC
	}
	// 4. Anything else is already plain text.
	return FormatText
}

// isLRCPlusUnits examines the units part of a timing-shaped line (§4.2):
// the first "<...>" holds a timestamp (v3) or a v1-bulk word:start:end entry.
func isLRCPlusUnits(s string) bool {
	if strings.HasPrefix(s, "{") {
		if j := indexByte(s, '}'); j >= 0 {
			s = strings.TrimSpace(s[j+1:])
		}
	}
	s = s[1:] // strip leading "<"
	if s == "" {
		return false
	}
	inner := s
	if j := indexByte(s, '>'); j >= 0 {
		inner = s[:j]
	}
	if inner == "" {
		return false
	}
	if inner[0] >= '0' && inner[0] <= '9' {
		// v3: start[,end] (plus legacy "|"/":" pairs) — the first token must
		// parse as a timestamp. v1 bulk lines can also open with a word that
		// happens to start with a digit, so fall through to that check too.
		if _, ok := parseTime(firstTimestampToken(inner)); ok {
			return true
		}
	}
	// v1 bulk: word:start:end|… (contains ':' and '|').
	return strings.Contains(inner, ":") && strings.Contains(inner, "|")
}

// firstTimestampToken extracts the leading timestamp from a v3 tspec like
// "00:01.31,00:01.66", "00:01.31:00:01.66" or "00:01.31|00:01.66".
func firstTimestampToken(tspec string) string {
	// The timestamp itself contains ":" — the pair separator follows the
	// fraction. Find the first ',' or '|' outside, else cut at the second ':'.
	if j := indexByte(tspec, ','); j >= 0 {
		return tspec[:j]
	}
	if j := indexByte(tspec, '|'); j >= 0 {
		return tspec[:j]
	}
	first := indexByte(tspec, ':')
	if first < 0 {
		return tspec
	}
	if j := indexByte(tspec[first+1:], ':'); j >= 0 {
		return tspec[:first+1+j]
	}
	return tspec
}

func looksLikeXML(s string) bool {
	if !strings.HasPrefix(s, "<") {
		return false
	}
	if strings.HasPrefix(s, "<?") || strings.HasPrefix(s, "<!") {
		return true
	}
	rest := s[1:]
	if rest == "" {
		return false
	}
	// "<" followed by a digit is an LRC+ timing unit, not a tag.
	return !(rest[0] >= '0' && rest[0] <= '9')
}
