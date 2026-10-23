// Package convert turns lyric files (LRC, LRC+, ELRC, TTML) into plain lines.
package convert

import (
	"strconv"
	"strings"
)

// Format is the input format detection result.
type Format string

const (
	FormatLRC     Format = "LRC"
	FormatLRCPlus Format = "LRC+"
	FormatELRC    Format = "ELRC"
	FormatTTML    Format = "TTML"
	FormatText    Format = "plaintext"
)

// Result is a converted lyric file.
type Result struct {
	Format Format
	Lines  []string
}

// Convert detects the format of input and converts it to plain lyric lines.
func Convert(input string) Result {
	lines := splitLines(input)
	format := Detect(lines)
	var out []string
	switch format {
	case FormatLRCPlus:
		out = fromLRCPlus(lines)
	case FormatELRC:
		out = fromELRC(lines)
	case FormatLRC:
		out = fromLRC(lines)
	case FormatTTML:
		out = fromTTML(input)
	default:
		out = passthrough(lines)
	}
	return Result{Format: format, Lines: out}
}

// passthrough trims empty leading/trailing lines from already-plain text.
func passthrough(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && lines[start] == "" {
		start++
	}
	for end > start && lines[end-1] == "" {
		end--
	}
	return lines[start:end]
}

// splitLines splits input into trimmed physical lines. CRLF and bare CR are
// normalized to LF first, since some terminals deliver pasted line breaks as
// carriage returns.
func splitLines(input string) []string {
	input = strings.TrimPrefix(input, "\uFEFF")
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	raw := strings.Split(input, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		out[i] = strings.TrimSpace(strings.TrimSuffix(l, "\r"))
	}
	return out
}

// parseTime parses an LRC/LRC+ timestamp ("m:ss", "mm:ss.xx", "mm:ss.xxx",
// "mm:ss:xx") into milliseconds.
func parseTime(s string) (ms int64, ok bool) {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return 0, false
	}
	rest := s[i+1:]
	sep, seps := -1, 0
	for k := 0; k < len(rest); k++ {
		if rest[k] == '.' || rest[k] == ':' {
			sep, seps = k, seps+1
		}
	}
	var secStr, frac string
	switch seps {
	case 0:
		secStr = rest
	case 1:
		secStr, frac = rest[:sep], rest[sep+1:]
	default:
		return 0, false
	}
	min, err1 := strconv.ParseInt(s[:i], 10, 64)
	sec, err2 := strconv.ParseInt(secStr, 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	var f int64
	if frac != "" {
		if len(frac) > 3 {
			return 0, false
		}
		for k := 0; k < len(frac); k++ {
			if frac[k] < '0' || frac[k] > '9' {
				return 0, false
			}
		}
		v, _ := strconv.ParseInt(frac, 10, 64)
		switch len(frac) {
		case 1:
			f = v * 100
		case 2:
			f = v * 10
		case 3:
			f = v
		}
	}
	return min*60000 + sec*1000 + f, true
}

// isTimestamp reports whether s has the shape of a lyric timestamp.
func isTimestamp(s string) bool {
	_, ok := parseTime(s)
	return ok
}

// parseHeader parses a lyric header line "[ts]text" (LRC/LRC+/ELRC).
// A legacy "{...}" attribute block between "]" and text is stripped (LRC+ Annex B).
func parseHeader(s string) (ms int64, text string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return 0, "", false
	}
	i := strings.IndexByte(s, ']')
	if i < 0 {
		return 0, "", false
	}
	ms, ok = parseTime(s[1:i])
	if !ok || ms < 0 {
		return 0, "", false
	}
	rest := strings.TrimSpace(s[i+1:])
	if strings.HasPrefix(rest, "{") {
		if j := indexByte(rest, '}'); j >= 0 {
			rest = strings.TrimSpace(rest[j+1:])
		}
	}
	return ms, rest, true
}

func indexByte(s string, b byte) int {
	for k := 0; k < len(s); k++ {
		if s[k] == b {
			return k
		}
	}
	return -1
}

// timingShaped reports whether s is an LRC+ timing line (§4.2):
// starts with "<", or a "{...}" line block followed by "<".
func timingShaped(s string) bool {
	if strings.HasPrefix(s, "<") {
		return true
	}
	if strings.HasPrefix(s, "{") {
		j := indexByte(s, '}')
		if j < 0 {
			return false
		}
		return strings.HasPrefix(strings.TrimSpace(s[j+1:]), "<")
	}
	return false
}

// entityReplacer decodes the LRC+ required entity set in spec order (§8).
var entityReplacer = strings.NewReplacer(
	"&#x27;", "'",
	"&apos;", "'",
	"&quot;", `"`,
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
)

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
