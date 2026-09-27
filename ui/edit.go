package ui

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glow/v3/utils"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Inline editing, Obsidian style: every block of the document stays rendered
// by Glamour except the one under the cursor, which turns into raw markdown
// you can type in. Changes are saved shortly after you stop typing.
//
// Local markdown files are also *viewed* block by block, so the view and the
// editor look the same and a click maps to the exact block under the mouse.

const (
	autosaveDelay = 300 * time.Millisecond
	// textarea hard-caps content at 10k lines; editing a longer file would
	// truncate it on the first autosave.
	maxEditLines = 10000
	promptWidth  = 2 // "│ ", same as Glamour's left margin
	// Typing without a pause this long is undone in one step.
	undoGroupGap = time.Second
)

// snapshot is one undo step: the whole document plus where the cursor was.
type snapshot struct {
	doc       string
	line, col int
}

type (
	openForEditMsg  *markdown
	editDebounceMsg int
)

// span is a block of the document: lines[start:end]. Blocks are separated by
// blank lines, except inside fenced code and frontmatter.
type span struct{ start, end int }

func findBlocks(lines []string) []span {
	var out []span
	start, fence := -1, ""
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if fence != "" {
			if (fence == "---" && (t == "---" || t == "...")) || (fence != "---" && strings.HasPrefix(t, fence)) {
				fence = ""
			}
			continue
		}
		if t == "" {
			if start >= 0 {
				out = append(out, span{start, i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
		switch {
		case i == 0 && t == "---":
			fence = "---"
		case strings.HasPrefix(t, "```"):
			fence = "```"
		case strings.HasPrefix(t, "~~~"):
			fence = "~~~"
		}
	}
	if start >= 0 {
		out = append(out, span{start, len(lines)})
	}
	if len(out) == 0 {
		out = []span{{0, 0}}
	}
	return out
}

// blockAt returns the block containing line, or the nearest one after it.
func blockAt(blocks []span, line int) int {
	for i, b := range blocks {
		if b.end > line {
			return i
		}
	}
	return len(blocks) - 1
}

// blockMode reports whether the document is shown block by block.
func (m *pagerModel) blockMode() bool {
	return m.currentDocument.localPath != "" &&
		utils.IsMarkdownFile(m.currentDocument.Note) &&
		m.common.cfg.GlamourEnabled &&
		!m.common.cfg.ShowLineNumbers
}

// showBlocks lays out the current document for viewing.
func (m *pagerModel) showBlocks() {
	m.lines = strings.Split(m.currentDocument.Body, "\n")
	m.blocks = findBlocks(m.lines)
	m.cur = -1
	m.resetRender()
	m.layout(false)
}

func (m *pagerModel) resetRender() {
	m.rendered = map[string][]string{}
	m.renderer = nil
}

// themeChanged re-renders everything after a light/dark switch.
func (m *pagerModel) themeChanged() tea.Cmd {
	m.resetRender()
	if m.editing {
		m.editor.SetStyles(editorStyles(m.common.isDark))
		m.layout(false)
		return nil
	}
	if m.currentDocument.Body == "" {
		return nil
	}
	return renderWithGlamour(*m, string(utils.RemoveFrontmatter([]byte(m.currentDocument.Body))))
}

func editorStyles(isDark bool) textarea.Styles {
	s := textarea.DefaultStyles(isDark)
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	return s
}

// startEditing opens block i for editing with the cursor on visual row `row`
// and column x (screen column, -1 for the start). screenTop is where the
// block's top should stay on screen, so nothing jumps.
func (m *pagerModel) startEditing(i, row, x, screenTop int) tea.Cmd {
	data, err := os.ReadFile(m.currentDocument.localPath)
	if err != nil {
		return m.showStatusMessage(pagerStatusMessage{"Can't edit: " + err.Error(), true})
	}
	body := string(data)
	if strings.Count(body, "\n") >= maxEditLines {
		return m.showStatusMessage(pagerStatusMessage{"File too long to edit inline", true})
	}

	ta := textarea.New()
	ta.SetStyles(editorStyles(m.common.isDark))
	ta.Prompt = "│ "
	ta.ShowLineNumbers = false
	ta.MaxHeight = 0
	ta.DynamicHeight = true

	m.editor = ta
	m.editing = true
	m.undo, m.redo = nil, nil
	m.slash = slashMenu{}
	m.sel = selection{}
	m.savedBody = body
	m.lines = strings.Split(body, "\n")
	m.blocks = findBlocks(m.lines)
	if m.rendered == nil {
		m.resetRender()
	}
	m.setSize(m.common.width, m.common.height)

	m.activate(min(max(i, 0), len(m.blocks)-1), 0, 0)
	m.placeCursor(row, x)
	m.layout(false)
	m.viewport.SetYOffset(m.editTop - screenTop)
	return m.editor.Focus()
}

// startEditingAtTop opens the first block visible on screen.
func (m *pagerModel) startEditingAtTop() tea.Cmd {
	if len(m.tops) == 0 {
		return m.startEditing(0, 0, -1, 1)
	}
	i, _ := m.hit(m.viewport.YOffset())
	return m.startEditing(i, 0, -1, m.tops[i]-m.viewport.YOffset())
}

// stopEditing saves any pending changes and goes back to the normal pager.
func (m *pagerModel) stopEditing() tea.Cmd {
	err := m.save()
	m.editing = false
	m.editor.Blur()
	m.setSize(m.common.width, m.common.height)
	if err != nil {
		return m.showStatusMessage(pagerStatusMessage{"Save failed: " + err.Error(), true})
	}
	return loadLocalMarkdown(&m.currentDocument)
}

func (m *pagerModel) blockText(b span) string {
	return strings.Join(m.lines[b.start:b.end], "\n")
}

// doc is the full document, including what's being typed right now.
func (m *pagerModel) doc() string {
	b := m.blocks[m.cur]
	return strings.Join(slices.Concat(m.lines[:b.start], []string{m.editor.Value()}, m.lines[b.end:]), "\n")
}

// save writes the document to disk if it changed.
func (m *pagerModel) save() error {
	if !m.editing {
		return nil
	}
	body := m.doc()
	if body == m.savedBody {
		return nil
	}
	if err := os.WriteFile(m.currentDocument.localPath, []byte(body), 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("saving %s: %w", m.currentDocument.localPath, err)
	}
	m.savedBody = body
	return nil
}

// commit writes the editor contents back into the document and re-splits it
// into blocks. It returns the line range the edited text now occupies.
func (m *pagerModel) commit() (start, end int) {
	b := m.blocks[m.cur]
	val := m.editor.Value()
	n := b.end - b.start
	if val != m.blockText(b) {
		newLines := strings.Split(val, "\n")
		m.lines = slices.Concat(m.lines[:b.start], newLines, m.lines[b.end:])
		m.blocks = findBlocks(m.lines)
		n = len(newLines)
	}
	return b.start, b.start + n
}

// activate makes block i editable and puts the cursor on row, col. A row or
// col of -1 means the last one.
func (m *pagerModel) activate(i, row, col int) {
	b := m.blocks[i]
	m.cur = i
	if row < 0 {
		row = b.end - b.start - 1
	}
	m.setEditor(m.blockText(b), row, col)
}

// setEditor fills the editor with val and puts the cursor on row, col.
func (m *pagerModel) setEditor(val string, row, col int) {
	m.editor.SetValue(val)
	m.editor.MoveToBegin()
	for n := 0; m.editor.Line() < row && n < maxEditLines; n++ {
		m.editor.CursorDown()
	}
	if col < 0 {
		col = maxEditLines
	}
	m.editor.SetCursorColumn(col)
}

// placeCursor puts the cursor on a visual (wrapped) row and screen column.
// ponytail: rendered and raw text line up only roughly (markup like ** is
// hidden when rendered), so a click can land a few characters off.
func (m *pagerModel) placeCursor(row, x int) {
	m.editor.MoveToBegin()
	for range row {
		m.editor.CursorDown()
	}
	m.editor.SetCursorColumn(m.editor.LineInfo().StartColumn + max(0, x-promptWidth))
}

// move goes to the next (dir > 0) or previous block.
func (m *pagerModel) move(dir, row, col int) {
	start, end := m.commit()
	i := -1
	for j, b := range m.blocks {
		if dir > 0 && b.start >= end {
			i = j
			break
		}
		if dir < 0 && b.end <= start {
			i = j
		}
	}
	if i < 0 {
		i = blockAt(m.blocks, start)
	}
	m.activate(i, row, col)
}

// joinPrevious removes the blank lines between this block and the one above,
// which is what backspace at the very start of a block means.
func (m *pagerModel) joinPrevious() {
	start, _ := m.commit()
	i := blockAt(m.blocks, start) - 1
	if i < 0 {
		return
	}
	prev := m.blocks[i]
	m.lines = slices.Delete(m.lines, prev.end, start)
	m.blocks = findBlocks(m.lines)
	m.activate(blockAt(m.blocks, prev.start), prev.end-prev.start, 0)
}

func (m *pagerModel) snap() snapshot {
	return snapshot{m.doc(), m.blocks[m.cur].start + m.editor.Line(), m.editor.Column()}
}

// restore puts the document and cursor back to a snapshot.
func (m *pagerModel) restore(s snapshot) {
	m.lines = strings.Split(s.doc, "\n")
	m.blocks = findBlocks(m.lines)
	i := blockAt(m.blocks, s.line)
	m.activate(i, max(0, s.line-m.blocks[i].start), s.col)
	m.lastEdit = time.Time{} // next edit starts a new undo step
}

// undoRedo pops a step off from and pushes the current state onto to.
func (m *pagerModel) undoRedo(from, to *[]snapshot) {
	if len(*from) == 0 {
		return
	}
	*to = append(*to, m.snap())
	s := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	m.restore(s)
}

// clickEdit moves the cursor to where the mouse clicked while editing.
func (m *pagerModel) clickEdit(x, y int) {
	i, row := m.hit(y + m.viewport.YOffset())
	if l, ok := m.taskAt(i, row, x); ok {
		m.undo, m.redo = append(m.undo, m.snap()), nil
		m.lastEdit = time.Time{}
		m.toggleTask(l)
		m.layout(false)
		return
	}
	screenTop := m.tops[i] - m.viewport.YOffset()
	if i != m.cur {
		target := m.blocks[i].start
		oldLen := m.blocks[m.cur].end - m.blocks[m.cur].start
		start, end := m.commit()
		if i > m.cur {
			target += (end - start) - oldLen
		}
		m.activate(blockAt(m.blocks, target), 0, 0)
	}
	m.placeCursor(row, x)
	m.layout(false)
	m.viewport.SetYOffset(m.editTop - screenTop)
}

func (m pagerModel) updateEditing(msg tea.Msg) (pagerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		ed := &m.editor
		li := ed.LineInfo()
		lastLine := ed.Line() == ed.LineCount()-1
		atTop := ed.Line() == 0 && li.RowOffset == 0
		atBottom := lastLine && li.RowOffset+1 >= li.Height
		atStart := ed.Line() == 0 && ed.Column() == 0
		value := ed.Value()
		atEnd := lastLine && ed.Column() == utf8.RuneCountInString(value[strings.LastIndex(value, "\n")+1:])
		first, last := m.cur == 0, m.cur == len(m.blocks)-1

		// Cmd+Z / Cmd+Shift+Z, with Ctrl and Ctrl+Y as fallbacks.
		isZ := msg.Code == 'z' || msg.Code == 'Z'
		mod := msg.Mod&(tea.ModSuper|tea.ModCtrl) != 0
		shift := msg.Mod&tea.ModShift != 0 || msg.Code == 'Z'
		before := m.snap()

		if m.slash.open && m.slashKey(msg) {
			m.layout(true)
			return m, m.scheduleSave()
		}

		var cmd tea.Cmd
		switch {
		case isZ && mod && !shift:
			m.undoRedo(&m.undo, &m.redo)
		case (isZ && mod && shift) || msg.String() == "ctrl+y":
			m.undoRedo(&m.redo, &m.undo)
		case msg.String() == keyEsc:
			return m, m.stopEditing()
		case msg.String() == "up" && atTop && !first:
			m.move(-1, -1, ed.Column())
		case msg.String() == "down" && atBottom && !last:
			m.move(1, 0, ed.Column())
		case msg.String() == "left" && atStart && !first:
			m.move(-1, -1, -1)
		case msg.String() == "right" && atEnd && !last:
			m.move(1, 0, 0)
		case msg.String() == "backspace" && atStart && !first:
			m.undo, m.redo = append(m.undo, before), nil
			m.lastEdit = time.Time{}
			m.joinPrevious()
		default:
			m.editor, cmd = m.editor.Update(msg)
			if m.doc() != before.doc {
				if time.Since(m.lastEdit) > undoGroupGap {
					m.undo = append(m.undo, before)
				}
				m.lastEdit = time.Now()
				m.redo = nil
			}
		}
		m.checkSlash(msg)
		m.layout(true)
		return m, tea.Batch(cmd, m.scheduleSave())

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && msg.Y < m.viewport.Height() {
			if m.slash.open {
				m.slash.open = false
				m.layout(false)
			}
			m.clickEdit(msg.X, msg.Y)
			return m, m.scheduleSave()
		}
		return m, nil

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case editDebounceMsg:
		if int(msg) != m.editGen {
			return m, nil // still typing
		}
		if err := m.save(); err != nil {
			return m, m.showStatusMessage(pagerStatusMessage{"Save failed: " + err.Error(), true})
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.resetRender()
		m.layout(true)
		return m, nil

	case statusMessageTimeoutMsg:
		m.state = pagerStateBrowse
		return m, nil

	case reloadMsg:
		return m, nil // our own autosave; the editor is the source of truth now
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	m.layout(false)
	return m, cmd
}

// scheduleSave saves after a short pause, if anything changed.
func (m *pagerModel) scheduleSave() tea.Cmd {
	if m.doc() == m.savedBody {
		return nil
	}
	m.editGen++
	gen := m.editGen
	return tea.Tick(autosaveDelay, func(time.Time) tea.Msg { return editDebounceMsg(gen) })
}

// renderBlock renders one block with Glamour, trimmed of blank edge lines.
func (m *pagerModel) renderBlock(i int) []string {
	text := m.blockText(m.blocks[i])
	if r, ok := m.rendered[text]; ok {
		return r
	}
	out := text
	if m.isFrontmatter(i) {
		// Glamour doesn't know frontmatter, show it as dim raw text.
		out = m.common.styles.subtleStyle.Render(indent(text, 2))
	} else {
		if m.renderer == nil {
			m.renderer, _ = glamour.NewTermRenderer(glamourOptions(*m, false)...)
		}
		if m.renderer != nil {
			if s, err := m.renderer.Render(text); err == nil {
				out = utils.ApplyTextSizing(s)
			}
		}
	}
	lines := boxTables(strings.Split(out, "\n"))
	blank := func(l string) bool { return strings.TrimSpace(ansi.Strip(l)) == "" }
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	m.rendered[text] = lines
	return lines
}

func (m *pagerModel) isFrontmatter(i int) bool {
	return m.blocks[i].start == 0 && strings.HasPrefix(m.lines[0], "---")
}

// layout rebuilds the screen. With follow set it scrolls to keep the cursor
// in view.
func (m *pagerModel) layout(follow bool) {
	out := []string{""}
	m.tops = make([]int, len(m.blocks))
	m.heights = make([]int, len(m.blocks))
	for i := range m.blocks {
		m.tops[i] = -1
		if !m.editing && m.isFrontmatter(i) {
			continue // hidden while viewing, like upstream Glow
		}
		if len(out) > 1 {
			out = append(out, "")
		}
		m.tops[i] = len(out)
		if m.editing && i == m.cur {
			m.editTop = len(out)
			ed := strings.Split(m.editor.View(), "\n")
			if m.slash.open {
				r := min(m.cursorRow()+1, len(ed))
				ed = slices.Concat(ed[:r], m.slashView(), ed[r:])
			}
			out = append(out, ed...)
		} else {
			out = append(out, m.renderBlock(i)...)
		}
		m.heights[i] = len(out) - m.tops[i]
	}
	m.viewport.SetContent(strings.Join(out, "\n"))
	if !follow || !m.editing {
		return
	}

	y, h := m.editTop+m.cursorRow(), m.viewport.Height()
	bottom := y
	if m.slash.open {
		bottom += len(m.slashView())
	}
	if y < m.viewport.YOffset() {
		m.viewport.SetYOffset(y)
	} else if bottom >= m.viewport.YOffset()+h {
		m.viewport.SetYOffset(bottom - h + 1)
	}
}

// cursorRow is the cursor's visual row inside the editor.
func (m *pagerModel) cursorRow() int {
	c := m.editor
	c.SetVirtualCursor(false)
	if cur := c.Cursor(); cur != nil {
		return cur.Y
	}
	return 0
}

// hit finds the block at content line y and the row within it. Clicks in the
// gap between blocks go to the block below.
func (m *pagerModel) hit(y int) (block, row int) {
	last := 0
	for i, top := range m.tops {
		if top < 0 {
			continue
		}
		if y < top {
			return i, 0
		}
		if y < top+m.heights[i] {
			return i, y - top
		}
		last = i
	}
	return last, max(0, m.heights[last]-1)
}

// editWidth matches the width Glamour wraps at, so raw and rendered text line up.
func (m *pagerModel) editWidth() int {
	w := m.viewport.Width()
	if mw := int(m.common.cfg.GlamourMaxWidth); mw > 0 { //nolint:gosec
		w = min(w, mw)
	}
	return w
}

var (
	taskSource   = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[[ xX]\]`)
	taskRendered = regexp.MustCompile(`^\s*\[[ ✓xX]\]`)
)

const doubleClickGap = 400 * time.Millisecond

// taskAt finds the to-do line whose rendered checkbox is at row, x of block i.
// Glamour draws each task's box at the start of its first row, so the k-th
// box row is the k-th task line in the source.
func (m *pagerModel) taskAt(i, row, x int) (int, bool) {
	if m.editing && i == m.cur {
		return 0, false // raw text; the click places the cursor instead
	}
	rows := m.renderBlock(i)
	if row >= len(rows) {
		return 0, false
	}
	plain := ansi.Strip(rows[row])
	loc := taskRendered.FindStringIndex(plain)
	if loc == nil {
		return 0, false
	}
	box := ansi.StringWidth(plain[:loc[1]]) - 3
	if x < box || x > box+3 {
		return 0, false
	}
	k := 0
	for _, r := range rows[:row] {
		if taskRendered.MatchString(ansi.Strip(r)) {
			k++
		}
	}
	b := m.blocks[i]
	for l := b.start; l < b.end; l++ {
		if taskSource.MatchString(m.lines[l]) {
			if k == 0 {
				return l, true
			}
			k--
		}
	}
	return 0, false
}

// toggleTask flips [ ] and [x] on line l.
func (m *pagerModel) toggleTask(l int) {
	s := m.lines[l]
	i := strings.Index(s, "[") + 1
	mark := "x"
	if s[i] != ' ' {
		mark = " "
	}
	m.lines[l] = s[:i] + mark + s[i+1:]
}

// clickView handles a click while viewing: a checkbox toggles, a double
// click starts editing right there.
func (m *pagerModel) clickView(x, y int) tea.Cmd {
	double := time.Since(m.lastClick) < doubleClickGap && y == m.lastClickY
	m.lastClick, m.lastClickY = time.Now(), y
	if len(m.tops) == 0 {
		return nil
	}
	i, row := m.hit(y + m.viewport.YOffset())
	if l, ok := m.taskAt(i, row, x); ok {
		m.toggleTask(l)
		body := strings.Join(m.lines, "\n")
		if err := os.WriteFile(m.currentDocument.localPath, []byte(body), 0o644); err != nil { //nolint:gosec
			return m.showStatusMessage(pagerStatusMessage{"Save failed: " + err.Error(), true})
		}
		m.currentDocument.Body = body
		m.layout(false)
		return nil
	}
	if double {
		return m.startEditing(i, row, x, m.tops[i]-m.viewport.YOffset())
	}
	return nil
}
