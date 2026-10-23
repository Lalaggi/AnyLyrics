package convert

import (
	"regexp"
	"strings"
)

// fromLRC extracts one line of text per header. Metadata lines ("[ti: …]"),
// voice-table lines ("@[…]"), section markers ("[Chorus]") and lines with no
// valid timestamp are skipped. Repeated leading timestamps ([ts][ts]Text)
// yield a single line, as for a classic LRC player.
func fromLRC(lines []string) []string {
	var out []string
	for _, s := range lines {
		text, ok := lrcHeaderText(s)
		if !ok || text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

// lrcHeaderText strips every leading "[timestamp]" group and a legacy "{…}"
// block from s.
func lrcHeaderText(s string) (string, bool) {
	if !strings.HasPrefix(s, "[") {
		return "", false
	}
	matched := false
	for strings.HasPrefix(s, "[") {
		i := indexByte(s, ']')
		if i < 0 || !isTimestamp(s[1:i]) {
			break
		}
		if ms, _ := parseTime(s[1:i]); ms < 0 {
			return "", false
		}
		matched = true
		s = s[i+1:]
	}
	if !matched {
		return "", false
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		if j := indexByte(s, '}'); j >= 0 {
			s = strings.TrimSpace(s[j+1:])
		}
	}
	return s, true
}

// fromLRCPlus extracts the header text of every block, in document order.
// Timing lines, the voice table, metadata and markers carry no display text.
func fromLRCPlus(lines []string) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		s := lines[i]
		if s == "" || s[0] == '@' || timingShaped(s) {
			continue
		}
		if _, text, ok := parseHeader(s); ok && text != "" {
			out = append(out, entityReplacer.Replace(text))
		}
	}
	return out
}

var (
	elrcTagRe   = regexp.MustCompile(`<\d+[0-9:.|,]*>`) // inline <mm:ss.xx> word tags
	elrcVoiceRe = regexp.MustCompile(`^([A-Za-z0-9_-]{1,16}):[ \t]*`)
)

// hasInlineWordTag reports whether text carries ELRC inline <mm:ss.xx> tags.
func hasInlineWordTag(text string) bool { return elrcTagRe.MatchString(text) }

// fromELRC strips the optional "id:" voice prefix and inline word timestamps
// ("[00:07.51]v2: <00:01.31>Why <00:01.66>couldn't").
func fromELRC(lines []string) []string {
	var out []string
	for _, s := range lines {
		text, ok := lrcHeaderText(s)
		if !ok {
			continue
		}
		if m := elrcVoiceRe.FindStringSubmatchIndex(text); m != nil {
			id := text[m[2]:m[3]]
			if strings.Contains(text, "<") || isVoiceId(id) {
				text = text[m[1]:]
			}
		}
		text = elrcTagRe.ReplaceAllString(text, "")
		if text = collapseWS(entityReplacer.Replace(text)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func isVoiceId(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for k := 0; k < len(s); k++ {
		if s[k] < '0' || s[k] > '9' {
			return false
		}
	}
	return true
}
