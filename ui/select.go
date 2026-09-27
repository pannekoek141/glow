package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Drag to select text while viewing; letting go copies it, like Claude Code.

// point is a spot in the content: line (not screen row) and column.
type point struct{ line, col int }

type selection struct {
	active, dragged bool // mouse is down / has moved since
	from, to        point
}

var selectStyle = lipgloss.NewStyle().Reverse(true)

func (m *pagerModel) contentPoint(x, y int) point {
	return point{y + m.viewport.YOffset(), x}
}

// ordered returns the selection's start and end, top first.
func (s selection) ordered() (point, point) {
	a, b := s.from, s.to
	if b.line < a.line || (b.line == a.line && b.col < a.col) {
		a, b = b, a
	}
	return a, b
}

// selectedText is the plain text under the selection.
func (m *pagerModel) selectedText() string {
	a, b := m.sel.ordered()
	lines := strings.Split(m.viewport.GetContent(), "\n")
	var out []string
	for l := a.line; l <= b.line && l < len(lines); l++ {
		from, to := 0, ansi.StringWidth(lines[l])
		if l == a.line {
			from = a.col
		}
		if l == b.line {
			to = b.col + 1
		}
		out = append(out, strings.TrimRight(ansi.Strip(ansi.Cut(lines[l], from, to)), " "))
	}
	// Drop Glamour's left margin: the first line starts where you pressed,
	// the rest lose their shared indent.
	out[0] = strings.TrimLeft(out[0], " ")
	indent := -1
	for _, l := range out[1:] {
		if t := strings.TrimLeft(l, " "); t != "" && (indent < 0 || len(l)-len(t) < indent) {
			indent = len(l) - len(t)
		}
	}
	for i := 1; i < len(out); i++ {
		out[i] = out[i][min(max(indent, 0), len(out[i])):]
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}

// selectMouse tracks press, drag and release. A release after a drag copies
// the selection; a plain click leaves nothing selected.
func (m *pagerModel) selectMouse(msg tea.MouseMsg) tea.Cmd {
	mouse := msg.Mouse()
	p := m.contentPoint(mouse.X, min(mouse.Y, m.viewport.Height()-1))
	switch msg.(type) {
	case tea.MouseClickMsg:
		m.sel = selection{active: true, from: p, to: p}
	case tea.MouseMotionMsg:
		if m.sel.active {
			m.sel.to, m.sel.dragged = p, m.sel.dragged || p != m.sel.from
		}
	case tea.MouseReleaseMsg:
		if !m.sel.active {
			return nil
		}
		m.sel.active = false
		if !m.sel.dragged {
			m.sel = selection{}
			return nil
		}
		text := m.selectedText()
		if text == "" {
			return nil
		}
		termenv.Copy(text)
		_ = clipboard.WriteAll(text)
		return m.showStatusMessage(pagerStatusMessage{fmt.Sprintf("Copied %d chars", len([]rune(text))), false})
	}
	return nil
}

// highlight draws the selection over the visible viewport lines.
func (m pagerModel) highlight(view string) string {
	if !m.sel.dragged {
		return view
	}
	a, b := m.sel.ordered()
	rows := strings.Split(view, "\n")
	for r := range rows {
		l := r + m.viewport.YOffset()
		if l < a.line || l > b.line {
			continue
		}
		from, to := 0, ansi.StringWidth(rows[r])
		if l == a.line {
			from = a.col
		}
		if l == b.line {
			to = min(b.col+1, to)
		}
		if from < to {
			rows[r] = lipgloss.StyleRanges(rows[r], lipgloss.NewRange(from, to, selectStyle))
		}
	}
	return strings.Join(rows, "\n")
}
