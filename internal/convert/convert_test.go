package convert

import (
	"reflect"
	"testing"
)

const lrcPlusExample = `[ti:Defying Gravity]
[ar:Kristin Chenoweth, Idina Menzel]
[al:Wicked (Original Broadway Cast Recording)]
@[v1,"Kristin Chenoweth",type=person]
@[v2,"Idina Menzel",type=person]
@[v1000,type=group]
@[v3,type=person]
[Intro]
[00:00.20]Elphaba
{v1}<00:00.20,00:00.97>Elphaba
[00:01.31]Why couldn't you have stayed calm for once
{v1}<00:01.31,00:01.66>Why <00:01.66,00:01.85>couldn't <00:01.85,00:01.95>you <00:01.95,00:02.05>have <00:02.05,00:02.37>stayed <00:02.37,00:02.69>calm <00:02.69,00:02.80>for <00:02.80,00:03.39>once
[Chorus]
[01:53.50]Defying gravity
{v2}<01:53.50,01:53.70>Def<01:53.70,01:54.96>y<01:54.96,01:55.26>ing <01:55.27,01:56.98>gravity
[02:00.00]Someone &amp; &lt;thing&gt; &quot;q&quot;
`

const plainLRC = `[ti:Test Song]
[ar:Artist]
[length:03:30]

[00:01.00][01:01.00]First line
[00:05.25]Second line
[Chorus]
[00:10.00]Third line
[by:someone]
`

const elrcExample = `[ti:ELRC Test]
[00:01.00]<00:01.00>Plain <00:01.30>word <00:01.60>timed
[00:05.00]v2: <00:05.00>Hello <00:05.30>world
[00:09.00]No tags at all
[00:11.00]v1:<00:11.00>No <00:11.50>space <00:12.00>prefix
`

const ttmlExample = `<?xml version="1.0" encoding="utf-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xmlns:itunes="http://music.apple.com/lyrics-ttml-internal">
 <head>
  <metadata>
   <agent xml:id="v1" type="person"><name>Idina Menzel</name></agent>
   <agent xml:id="v2" type="person"><name>Kristin Chenoweth</name></agent>
  </metadata>
 </head>
 <body>
  <div>
   <p begin="00:00.000" end="00:01.500" agent="v1"><span begin="00:00.000" end="00:00.500">Def</span><span begin="00:00.500" end="00:00.900">y</span><span begin="00:00.900" end="00:01.100">ing </span><span begin="00:01.100" end="00:01.500">gravity</span></p>
   <p begin="00:02.000" end="00:03.500" agent="v2"><span begin="00:02.000" end="00:02.500">Something </span><span begin="00:02.500" end="00:03.000">beautiful</span></p>
  </div>
 </body>
</tt>
`

func TestLRCPlus(t *testing.T) {
	res := Convert(lrcPlusExample)
	if res.Format != FormatLRCPlus {
		t.Fatalf("format = %s, want LRC+", res.Format)
	}
	want := []string{
		"Elphaba",
		"Why couldn't you have stayed calm for once",
		"Defying gravity",
		`Someone & <thing> "q"`,
	}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines =\n%q\nwant\n%q", res.Lines, want)
	}
}

func TestLRC(t *testing.T) {
	res := Convert(plainLRC)
	if res.Format != FormatLRC {
		t.Fatalf("format = %s, want LRC", res.Format)
	}
	want := []string{"First line", "Second line", "Third line"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
}

func TestELRC(t *testing.T) {
	res := Convert(elrcExample)
	if res.Format != FormatELRC {
		t.Fatalf("format = %s, want ELRC", res.Format)
	}
	want := []string{
		"Plain word timed",
		"Hello world",
		"No tags at all",
		"No space prefix",
	}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
}

func TestTTML(t *testing.T) {
	res := Convert(ttmlExample)
	if res.Format != FormatTTML {
		t.Fatalf("format = %s, want TTML", res.Format)
	}
	want := []string{
		"Defying gravity",
		"Something beautiful",
	}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
}

func TestCRLineEndings(t *testing.T) {
	// Some terminals deliver pasted line breaks as bare CR.
	res := Convert("[ti:X]\r[00:01.00]First\r[00:02.00]Second\r")
	if res.Format != FormatLRC || len(res.Lines) != 2 {
		t.Fatalf("got format=%s lines=%q", res.Format, res.Lines)
	}
	if res.Lines[0] != "First" || res.Lines[1] != "Second" {
		t.Errorf("lines = %q", res.Lines)
	}
}

func TestPlainText(t *testing.T) {
	res := Convert("just some\nlyrics\n")
	if res.Format != FormatText {
		t.Fatalf("format = %s, want plaintext", res.Format)
	}
	want := []string{"just some", "lyrics"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
}

func TestTimestamps(t *testing.T) {
	cases := []struct {
		in string
		ms int64
		ok bool
	}{
		{"00:01.3", 1300, true},
		{"00:01.31", 1310, true},
		{"00:01.318", 1318, true},
		{"12:34", 754000, true},
		{"1:02:03.5", 0, false}, // exactly one ':' required
	}
	for _, c := range cases {
		ms, ok := parseTime(c.in)
		if ms != c.ms || ok != c.ok {
			t.Errorf("parseTime(%q) = %d,%v want %d,%v", c.in, ms, ok, c.ms, c.ok)
		}
	}
}
