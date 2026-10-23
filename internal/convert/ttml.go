package convert

import (
	"encoding/xml"
	"io"
	"strings"
)

// fromTTML extracts one line of text per <p> element, joining timed sibling
// spans whitespace-aware (LRC+ v3 Annex A): spans separated by whitespace are
// separate words, adjacent spans glue into one word.
//
// Lines without begin are kept only in fully unsynced documents. Background
// spans (role "x-bg") stay inline — plain text has no voice separation.
func fromTTML(input string) []string {
	type pending struct {
		text string
		sync bool
	}
	dec := xml.NewDecoder(strings.NewReader(input))
	dec.Strict = false
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // assume UTF-8
	}

	var (
		pendings []pending
		cur      *strings.Builder
		depth    int // element nesting inside the current <p>
		sync     bool
		sawSync  bool
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // tolerate truncated pastes
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if cur == nil {
				if t.Name.Local == "p" {
					cur = &strings.Builder{}
					depth = 1
					sync = attr(t, "begin") != ""
				}
				continue
			}
			depth++
			if t.Name.Local == "br" {
				cur.WriteByte('\n')
			}
		case xml.EndElement:
			if cur == nil {
				continue
			}
			depth--
			if depth == 0 {
				text := strings.TrimSpace(cur.String())
				// Collapse per-line whitespace but keep <br> breaks out of
				// scope: a <br> is rare; treat it as a plain space.
				text = collapseWS(strings.ReplaceAll(text, "\n", " "))
				pendings = append(pendings, pending{text, sync})
				cur = nil
				if sync {
					sawSync = true
				}
			}
		case xml.CharData:
			if cur != nil {
				cur.Write(t)
			}
		}
	}

	var out []string
	for _, p := range pendings {
		if sawSync && !p.sync {
			continue // stray non-timed <p> in a synced document
		}
		if p.text != "" {
			out = append(out, p.text)
		}
	}
	return out
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}
