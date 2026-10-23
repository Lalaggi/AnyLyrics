package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"anylyrics/internal/convert"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	badgeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114"))
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	textStyle   = lipgloss.NewStyle()
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("248"))
)

type state int

const (
	stateInput state = iota
	stateOutput
	stateError
)

type model struct {
	state  state
	buf    string // pasted input
	result convert.Result
	err    error
	width  int
	height int
	offset int // output scroll offset, in wrapped rows

	status    string // transient toast ("copied")
	statusGen int
	quitCopy  bool // copy-and-exit requested
}

func newModel() model { return model{state: stateInput} }

func (m model) Init() tea.Cmd { return nil }

// copiedMsg reports the result of a clipboard copy.
type copiedMsg struct{ err error }

// clearStatusMsg clears a toast if it is still current.
type clearStatusMsg struct{ gen int }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()
		return m, nil

	case copiedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		if m.quitCopy {
			return m, tea.Quit
		}
		m.status = "copied ✓"
		m.statusGen++
		gen := m.statusGen
		return m, tea.Tick(1200*time.Millisecond, func(time.Time) tea.Msg {
			return clearStatusMsg{gen: gen}
		})

	case clearStatusMsg:
		if msg.gen == m.statusGen {
			m.status = ""
		}
		return m, nil

	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			if m.state == stateOutput {
				m.quitCopy = false
				return m, copyCmd(m.plaintext())
			}
		case msg.Button == tea.MouseButtonWheelUp && m.state == stateOutput:
			m.offset--
			m.clampOffset()
		case msg.Button == tea.MouseButtonWheelDown && m.state == stateOutput:
			m.offset++
			m.clampOffset()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case clipMsg:
		if msg.err != nil {
			m.status = "clipboard unavailable"
			m.statusGen++
			gen := m.statusGen
			return m, tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
				return clearStatusMsg{gen: gen}
			})
		}
		if m.state == stateInput {
			m.buf = normalizeNewlines(m.buf + msg.text)
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch m.state {
	case stateInput:
		switch msg.Type {
		// A pasted newline arrives inside a KeyRunes message, so a real
		// Enter is unambiguous. Terminals report Ctrl+Enter as either
		// CR (enter) or LF (ctrl+j); both submit.
		case tea.KeyEnter, tea.KeyCtrlJ:
			return m.submit()
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyBackspace:
			if r := []rune(m.buf); len(r) > 0 {
				m.buf = string(r[:len(r)-1])
			}
		case tea.KeyCtrlU:
			m.buf = ""
		case tea.KeyCtrlV:
			return m, pasteClipboardCmd()
		case tea.KeyRunes, tea.KeySpace:
			if !msg.Alt {
				m.buf = normalizeNewlines(m.buf + string(msg.Runes))
			}
		}
		return m, nil

	case stateOutput:
		switch msg.Type {
		case tea.KeyEnter:
			m.quitCopy = true
			return m, copyCmd(m.plaintext())
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			m.offset--
			m.clampOffset()
		case tea.KeyDown:
			m.offset++
			m.clampOffset()
		case tea.KeyPgUp:
			m.offset -= m.bodyRows()
			m.clampOffset()
		case tea.KeyPgDown:
			m.offset += m.bodyRows()
			m.clampOffset()
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "q":
				return m, tea.Quit
			case "c":
				m.quitCopy = false
				return m, copyCmd(m.plaintext())
			case "k":
				m.offset--
				m.clampOffset()
			case "j":
				m.offset++
				m.clampOffset()
			case "g":
				m.offset = 0
			case "G":
				m.offset = len(m.wrapRows())
				m.clampOffset()
			}
		}
		return m, nil

	case stateError:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyCtrlJ, tea.KeyEsc:
			m.state = stateInput
		case tea.KeyRunes:
			if string(msg.Runes) == "q" {
				return m, tea.Quit
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) submit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.buf)
	if input == "" {
		return m, nil
	}
	res := convert.Convert(input)
	if len(res.Lines) == 0 {
		m.err = fmt.Errorf("no lyrics found in the pasted text")
		m.state = stateError
		return m, nil
	}
	m.result = res
	m.state = stateOutput
	m.offset = 0
	m.status = ""
	return m, nil
}

func (m model) plaintext() string {
	return strings.Join(m.result.Lines, "\n")
}

// ── geometry ─────────────────────────────────────────────────────────

const maxBoxWidth = 100

// boxWidth is the rendered width of the bordered content box.
func (m model) boxWidth() int {
	w := m.width - 4
	if w > maxBoxWidth {
		w = maxBoxWidth
	}
	if w < 24 {
		w = 24
	}
	return w
}

// innerWidth is the usable text width inside the box.
func (m model) innerWidth() int {
	return m.boxWidth() - 4 // 2 border + 1 padding on each side
}

// bodyRows is the number of content rows that fit in the box. Every view
// uses the same chrome (header, subtitle, blank, box borders, blank, help),
// so switching views never shifts the layout.
func (m model) bodyRows() int {
	h := m.height - 8
	if h < 1 {
		h = 1
	}
	return h
}

// wrapRows wraps the converted lyrics to the inner width.
func (m model) wrapRows() []string {
	width := m.innerWidth()
	var rows []string
	for _, line := range m.result.Lines {
		r := []rune(line)
		for {
			if len(r) <= width {
				rows = append(rows, string(r))
				break
			}
			rows = append(rows, string(r[:width]))
			r = r[width:]
		}
	}
	return rows
}

func (m *model) clampOffset() {
	max := len(m.wrapRows()) - m.bodyRows()
	if max < 0 {
		max = 0
	}
	if m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// ── views ────────────────────────────────────────────────────────────

func (m model) View() string {
	switch m.state {
	case stateOutput:
		return m.outputView()
	case stateError:
		return m.errorView()
	}
	return m.inputView()
}

func (m model) header(right string) string {
	left := accentStyle.Render("anylyrics")
	if right != "" {
		gap := m.boxWidth() - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		return left + strings.Repeat(" ", gap) + right
	}
	return left
}

// box renders content inside a rounded border of a fixed width and height.
func (m model) box(content []string, rows int) string {
	inner := m.innerWidth()
	lines := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		lines = append(lines, textStyle.Render(padRight(line, inner)))
	}
	return borderStyle.Render("╭"+strings.Repeat("─", inner+2)+"╮") + "\n" +
		borderStyle.Render("│ ") + strings.Join(lines, borderStyle.Render(" │\n│ ")) + borderStyle.Render(" │") + "\n" +
		borderStyle.Render("╰"+strings.Repeat("─", inner+2)+"╯")
}

func (m model) help(parts ...string) string {
	return mutedStyle.Render(strings.Join(parts, "  ·  "))
}

func (m model) inputView() string {
	var b strings.Builder
	b.WriteString(m.header(""))
	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(errStyle.Render(m.status))
	} else {
		b.WriteString(mutedStyle.Render("paste a lyric file — LRC, LRC+, ELRC or TTML"))
	}
	b.WriteString("\n\n")

	rows := m.bodyRows()
	var content []string
	if m.buf == "" {
		content = []string{mutedStyle.Render("waiting for a paste…")}
	} else {
		lines := strings.Split(strings.TrimRight(m.buf, "\n"), "\n")
		var wrapped []string
		for _, l := range lines {
			wrapped = append(wrapped, wrapText(l, m.innerWidth())...)
		}
		start := len(wrapped) - rows
		if start < 0 {
			start = 0
		}
		content = wrapped[start:]
		if start > 0 {
			content = append([]string{mutedStyle.Render(fmt.Sprintf("… %d earlier lines", start))}, content...)
			if len(content) > rows {
				content = content[1:]
			}
		}
	}
	b.WriteString(m.box(content, rows))
	b.WriteString("\n\n")
	b.WriteString(m.help(
		keyStyle.Render("ctrl+enter")+mutedStyle.Render(" convert"),
		keyStyle.Render("ctrl+v")+mutedStyle.Render(" paste"),
		keyStyle.Render("ctrl+u")+mutedStyle.Render(" clear"),
		keyStyle.Render("ctrl+c")+mutedStyle.Render(" quit"),
	))
	return b.String()
}

func (m model) outputView() string {
	right := ""
	switch {
	case m.status != "":
		right = badgeStyle.Render(m.status)
	case m.offset > 0 || m.offset < len(m.wrapRows())-m.bodyRows():
		right = mutedStyle.Render(fmt.Sprintf("%d/%d", m.offset+1, len(m.wrapRows())))
	}

	var b strings.Builder
	b.WriteString(m.header(right))
	b.WriteString("\n")
	b.WriteString(badgeStyle.Render(string(m.result.Format)) + "  " +
		mutedStyle.Render(fmt.Sprintf("%d lines", len(m.result.Lines))))
	b.WriteString("\n\n")

	rows := m.wrapRows()
	content := make([]string, 0, m.bodyRows())
	for i := m.offset; i < len(rows) && i < m.offset+m.bodyRows(); i++ {
		content = append(content, rows[i])
	}
	b.WriteString(m.box(content, m.bodyRows()))
	b.WriteString("\n\n")
	b.WriteString(m.help(
		keyStyle.Render("enter")+mutedStyle.Render(" copy & quit"),
		keyStyle.Render("c")+mutedStyle.Render(" copy"),
		keyStyle.Render("click")+mutedStyle.Render(" copy"),
		keyStyle.Render("↑↓")+mutedStyle.Render(" scroll"),
		keyStyle.Render("q")+mutedStyle.Render(" quit"),
	))
	return b.String()
}

func (m model) errorView() string {
	var b strings.Builder
	b.WriteString(m.header(""))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("couldn't convert that"))
	b.WriteString("\n\n")
	msg := "unexpected error"
	if m.err != nil {
		msg = m.err.Error()
	}
	b.WriteString(m.box([]string{errStyle.Render(msg)}, m.bodyRows()))
	b.WriteString("\n\n")
	b.WriteString(m.help(
		keyStyle.Render("enter")+mutedStyle.Render(" try again"),
		keyStyle.Render("q")+mutedStyle.Render(" quit"),
	))
	return b.String()
}

// padRight pads s with spaces to width w (rune-count aware).
func padRight(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// wrapText hard-wraps a single line to width runes.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	r := []rune(s)
	if len(r) == 0 {
		return []string{""}
	}
	var out []string
	for len(r) > width {
		out = append(out, string(r[:width]))
		r = r[width:]
	}
	return append(out, string(r))
}

// normalizeNewlines converts CRLF and bare CR to LF, so pasted content that
// arrives with carriage returns displays and converts correctly.
func normalizeNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// clipMsg carries the result of reading the Wayland clipboard.
type clipMsg struct {
	text string
	err  error
}

// pasteClipboardCmd reads the clipboard via wl-paste for ctrl+v, since some
// terminals reserve ctrl+v for "paste raw" (or nothing at all).
func pasteClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("wl-paste", "--no-newline").Output()
		return clipMsg{text: string(out), err: err}
	}
}

// copyCmd shells out to wl-copy with the plaintext as its stdin.
func copyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		return copiedMsg{err: cmd.Run()}
	}
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Println("anylyrics:", err)
	}
}
