package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/glow/v3/utils"
)

// New file from the file listing: press n, type a name, hit enter, and the
// file opens straight into edit mode.

func newNameInput(common *commonModel) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "New file:"
	ti.Placeholder = " notes/idea.md"
	ti.SetVirtualCursor(true)
	s := ti.Styles()
	s.Focused.Prompt = common.styles.stashInputPromptStyle
	s.Cursor.Color = common.styles.fuchsia
	ti.SetStyles(s)
	return ti
}

func (m *stashModel) startNaming() tea.Cmd {
	m.nameInput = newNameInput(m.common)
	m.nameInput.SetWidth(m.filterInput.Width())
	// Start in the folder of the selected file.
	if sel := m.selectedMarkdown(); sel != nil {
		if dir := filepath.Dir(sel.Note); dir != "." {
			m.nameInput.SetValue(dir + string(filepath.Separator))
			m.nameInput.CursorEnd()
		}
	}
	m.naming = true
	return m.nameInput.Focus()
}

func (m *stashModel) handleNaming(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case keyEsc:
			m.naming = false
			return nil
		case keyEnter:
			m.naming = false
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				return nil
			}
			md, err := m.createFile(name)
			if err != nil {
				return m.showError("Can't create file: " + err.Error())
			}
			return func() tea.Msg { return openForEditMsg(md) }
		}
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return cmd
}

// createFile creates name (relative to the listed folder) and adds it to the
// listing. An existing file is simply opened.
func (m *stashModel) createFile(name string) (*markdown, error) {
	root := m.common.cwd
	if root == "" {
		root, _ = os.Getwd()
	}
	if filepath.Ext(name) == "" || !utils.IsMarkdownFile(name) {
		name += ".md"
	}
	path := filepath.Join(root, name)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec
		return nil, err //nolint:wrapcheck
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec
	switch {
	case errors.Is(err, fs.ErrExist):
		for _, md := range m.markdowns {
			if md.localPath == path {
				return md, nil
			}
		}
	case err != nil:
		return nil, err //nolint:wrapcheck
	default:
		_ = f.Close()
	}

	md := &markdown{localPath: path, Note: stripAbsolutePath(path, root), Modtime: time.Now()}
	m.addMarkdowns(md)
	return md, nil
}

func (m *stashModel) showError(msg string) tea.Cmd {
	m.showStatusMessage = true
	m.statusMessage = statusMessage{errorStatusMessage, msg}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.statusMessageTimer = time.NewTimer(statusMessageTimeout)
	return waitForStatusMessageTimeout(stashContext, m.statusMessageTimer)
}

// trashSelected moves the selected file to ~/.Trash and drops it from the list.
func (m *stashModel) trashSelected() tea.Cmd {
	md := m.selectedMarkdown()
	if md == nil || md.localPath == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return m.showError("Can't delete: " + err.Error())
	}
	name := filepath.Base(md.localPath)
	dst := filepath.Join(home, ".Trash", name)
	if _, err := os.Stat(dst); err == nil {
		dst = filepath.Join(home, ".Trash", time.Now().Format("150405 ")+name)
	}
	// ponytail: plain rename, fails across disks; use Finder via osascript if that bites.
	if err := os.Rename(md.localPath, dst); err != nil {
		return m.showError("Can't delete: " + err.Error())
	}
	gone := func(x *markdown) bool { return x == md }
	m.markdowns = slices.DeleteFunc(m.markdowns, gone)
	m.filteredMarkdowns = slices.DeleteFunc(m.filteredMarkdowns, gone)
	m.updatePagination()
	m.setCursor(max(0, min(m.cursor(), m.paginator().ItemsOnPage(len(m.getVisibleMarkdowns()))-1)))
	return nil
}
