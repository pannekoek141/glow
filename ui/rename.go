package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/glow/v3/utils"
	"github.com/charmbracelet/log"
)

// Rename the open file: double-click its name in the status bar, edit it,
// enter to rename, esc to cancel.

type fileRenamedMsg struct{ from, to, note string }

// clickStatusBar starts renaming on a double click.
func (m *pagerModel) clickStatusBar() tea.Cmd {
	y := m.viewport.Height()
	double := time.Since(m.lastClick) < doubleClickGap && m.lastClickY == y
	m.lastClick, m.lastClickY = time.Now(), y
	if !double || m.currentDocument.localPath == "" {
		return nil
	}
	m.nameInput = newNameInput(m.common)
	m.nameInput.Prompt = "Rename: "
	m.nameInput.Placeholder = ""
	s := m.nameInput.Styles()
	s.Cursor.Blink = false // blink ticks would go to the pager, not the input
	m.nameInput.SetStyles(s)
	m.nameInput.SetWidth(max(10, m.common.width-30))
	m.nameInput.SetValue(m.currentDocument.Note)
	m.nameInput.CursorEnd()
	m.renaming = true
	return m.nameInput.Focus()
}

// updateRenaming handles keys while the name is being edited. It reports
// false for messages that aren't for the name input.
func (m *pagerModel) updateRenaming(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyEsc:
			m.renaming = false
			return nil, true
		case keyEnter:
			m.renaming = false
			return m.rename(strings.TrimSpace(m.nameInput.Value())), true
		}
	case tea.PasteMsg:
	default:
		return nil, false
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return cmd, true
}

// rename moves the file to name, relative to the folder glow was opened in.
func (m *pagerModel) rename(name string) tea.Cmd {
	if name == "" || name == m.currentDocument.Note {
		return nil
	}
	if filepath.Ext(name) == "" || !utils.IsMarkdownFile(name) {
		name += ".md"
	}
	root := m.common.cwd
	if root == "" {
		root, _ = os.Getwd()
	}
	to := name
	if !filepath.IsAbs(to) {
		to = filepath.Join(root, name)
	}
	from := m.currentDocument.localPath
	fail := func(err error) tea.Cmd {
		return m.showStatusMessage(pagerStatusMessage{"Can't rename: " + err.Error(), true})
	}
	// A case-only rename on macOS finds the file itself; that's fine.
	if fi, err := os.Stat(to); err == nil && !sameFile(fi, from) {
		return fail(errors.New(name + " already exists"))
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil { //nolint:gosec
		return fail(err)
	}
	if err := os.Rename(from, to); err != nil {
		return fail(err)
	}
	m.follow(from, to)
	m.currentDocument.localPath = to
	m.currentDocument.Note = stripAbsolutePath(to, root)
	note := m.currentDocument.Note
	return tea.Batch(
		m.showStatusMessage(pagerStatusMessage{"Renamed to " + note, false}),
		func() tea.Msg { return fileRenamedMsg{from, to, note} },
	)
}

func sameFile(fi os.FileInfo, path string) bool {
	other, err := os.Stat(path)
	return err == nil && os.SameFile(fi, other)
}

// follow points the file watcher at the renamed file.
func (m *pagerModel) follow(from, to string) {
	if m.watcher == nil {
		return
	}
	if dir := filepath.Dir(to); dir != filepath.Dir(from) {
		_ = m.watcher.Remove(filepath.Dir(from))
		if err := m.watcher.Add(dir); err != nil {
			log.Error("error adding dir to fsnotify watcher", "error", err)
		}
	}
	m.watched.Store(&to)
}
