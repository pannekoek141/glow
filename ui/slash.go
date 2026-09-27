package ui

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"
)

// Slash menu, Notion style: type / at the start of a line (or after a space)
// while editing, pick a markdown element, and it's inserted for you.

// cursorMark is where the cursor goes after inserting a snippet.
const cursorMark = "\x00"

const slashMenuRows = 8

type slashItem struct {
	name, hint, text string
	block            bool // needs its own line
}

var slashItems = []slashItem{
	{"Heading 1", "#", "# \x00", true},
	{"Heading 2", "##", "## \x00", true},
	{"Heading 3", "###", "### \x00", true},
	{"Bullet list", "-", "- \x00", true},
	{"Numbered list", "1.", "1. \x00", true},
	{"To-do list", "[ ]", "- [ ] \x00", true},
	{"Quote", ">", "> \x00", true},
	{"Code block", "```", "```\n\x00\n```", true},
	{"Table", "|", "| \x00 |  |\n| --- | --- |\n|  |  |", true},
	{"Divider", "***", "***\x00", true},
	{"Bold", "**", "**\x00**", false},
	{"Italic", "*", "*\x00*", false},
	{"Strikethrough", "~~", "~~\x00~~", false},
	{"Inline code", "`", "`\x00`", false},
	{"Link", "[]()", "[\x00](url)", false},
	{"Image", "![]()", "![\x00](url)", false},
}

// slashMenu is the open menu: the / sits at line, col of block cur.
type slashMenu struct {
	open           bool
	cur, line, col int
	sel            int
}

func (m *pagerModel) editorLine() []rune {
	return []rune(strings.Split(m.editor.Value(), "\n")[m.editor.Line()])
}

func (m *pagerModel) slashQuery() string {
	rs := m.editorLine()
	return string(rs[min(m.slash.col+1, len(rs)):max(m.slash.col+1, min(m.editor.Column(), len(rs)))])
}

func (m *pagerModel) slashMatches() []slashItem {
	q := m.slashQuery()
	if q == "" {
		return slashItems
	}
	names := make([]string, len(slashItems))
	for i, it := range slashItems {
		names[i] = it.name
	}
	var out []slashItem
	for _, r := range fuzzy.Find(q, names) {
		out = append(out, slashItems[r.Index])
	}
	return out
}

// checkSlash opens the menu right after a / was typed, and closes it once the
// cursor leaves the /query text.
func (m *pagerModel) checkSlash(k tea.KeyPressMsg) {
	rs, col := m.editorLine(), m.editor.Column()
	if k.Text == "/" && col > 0 && col <= len(rs) && rs[col-1] == '/' && (col == 1 || unicode.IsSpace(rs[col-2])) {
		m.slash = slashMenu{open: true, cur: m.cur, line: m.editor.Line(), col: col - 1}
		return
	}
	if !m.slash.open {
		return
	}
	if m.cur != m.slash.cur || m.editor.Line() != m.slash.line || col <= m.slash.col ||
		m.slash.col >= len(rs) || rs[m.slash.col] != '/' ||
		strings.ContainsRune(m.slashQuery(), ' ') || len(m.slashMatches()) == 0 {
		m.slash.open = false
		return
	}
	m.slash.sel = min(m.slash.sel, len(m.slashMatches())-1)
}

// slashKey handles a key while the menu is open. It reports false for keys
// the editor should get instead.
func (m *pagerModel) slashKey(k tea.KeyPressMsg) bool {
	n := len(m.slashMatches())
	switch k.String() {
	case "up", "ctrl+p":
		m.slash.sel = (m.slash.sel + n - 1) % n
	case "down", "ctrl+n":
		m.slash.sel = (m.slash.sel + 1) % n
	case keyEnter, "tab":
		m.undo, m.redo = append(m.undo, m.snap()), nil
		m.lastEdit = time.Time{} // next typing is its own undo step
		m.applySlash(m.slashMatches()[m.slash.sel])
		m.slash.open = false
	case keyEsc:
		m.slash.open = false
	default:
		return false
	}
	return true
}

// applySlash replaces the /query with the item's snippet.
func (m *pagerModel) applySlash(it slashItem) {
	lines := strings.Split(m.editor.Value(), "\n")
	rs := []rune(lines[m.slash.line])
	before, after := string(rs[:m.slash.col]), string(rs[min(m.editor.Column(), len(rs)):])
	snip := it.text
	if it.block && strings.TrimSpace(before) != "" {
		snip = "\n" + snip
	}
	lines[m.slash.line] = before + snip + after
	val := strings.Join(lines, "\n")
	i := strings.Index(val, cursorMark)
	row := strings.Count(val[:i], "\n")
	col := utf8.RuneCountInString(val[strings.LastIndex(val[:i], "\n")+1 : i])
	m.setEditor(strings.Replace(val, cursorMark, "", 1), row, col)
}

func (m *pagerModel) slashView() []string {
	s := m.common.styles
	items := m.slashMatches()
	top := max(0, m.slash.sel-slashMenuRows+1)
	const w = 28
	var rows []string
	for i := top; i < min(len(items), top+slashMenuRows); i++ {
		it := items[i]
		pad := max(1, w-len(it.name)-len(it.hint))
		row := " " + it.name + strings.Repeat(" ", pad) + s.subtleStyle.Render(it.hint) + " "
		if i == m.slash.sel {
			row = lipgloss.NewStyle().Background(s.fuchsia).Foreground(lipgloss.Color("#FFFDF5")).
				Render(" " + it.name + strings.Repeat(" ", pad) + it.hint + " ")
		}
		rows = append(rows, row)
	}
	rows = append(rows, s.subtleStyle.Render(" ↑↓ pick • enter insert • esc"))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(s.fuchsia).
		MarginLeft(promptWidth).Render(strings.Join(rows, "\n"))
	return strings.Split(box, "\n")
}
