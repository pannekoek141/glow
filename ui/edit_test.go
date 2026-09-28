package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func TestFindBlocks(t *testing.T) {
	doc := "---\ntitle: x\n\nmore: y\n---\n\n# Head\n\npara\nline two\n\n\n```\ncode\n\ncode\n```\n"
	got := findBlocks(strings.Split(doc, "\n"))
	want := []span{{0, 5}, {6, 7}, {8, 10}, {12, 17}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := findBlocks([]string{""}); !reflect.DeepEqual(got, []span{{0, 0}}) {
		t.Fatalf("empty doc: got %v", got)
	}
}

func TestApplySlash(t *testing.T) {
	m := pagerModel{editor: textarea.New()}
	for _, tc := range []struct{ line, pick, want string }{
		{"/bo", "Bold", "**x**"},
		{"hi /h1", "Heading 1", "hi \n# x"},
		{"/co", "Code block", "```\nx\n```"},
	} {
		m.setEditor(tc.line, 0, len(tc.line))
		m.slash = slashMenu{open: true, col: strings.Index(tc.line, "/")}
		i := slices.IndexFunc(slashItems, func(it slashItem) bool { return it.name == tc.pick })
		m.applySlash(slashItems[i])
		m.editor.InsertString("x")
		if got := m.editor.Value(); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestBoxTables(t *testing.T) {
	in := []string{"   A │ B ", "  ───┼───", "   1 │ 2 ", "", "  text"}
	want := []string{" ┌───┬──┐", " │ A │ B│", " ├───┼──┤", " │ 1 │ 2│", " └───┴──┘", "", "  text"}
	if got := boxTables(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func TestSelectedText(t *testing.T) {
	m := pagerModel{viewport: viewport.New()}
	m.viewport.SetContent("  First line   \n\n  Second\n    nested")
	m.sel = selection{from: point{0, 4}, to: point{3, 7}}
	if got, want := m.selectedText(), "rst line\n\nSecond\n  nest"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestContinueList(t *testing.T) {
	m := pagerModel{editor: textarea.New()}
	m.editor.MaxHeight = 0
	for _, tc := range []struct{ in, want string }{
		{"- one", "- one\n- "},
		{"  * [x] done", "  * [x] done\n  * [ ] "},
		{"9. nine", "9. nine\n10. "},
		{"- one\n- ", "- one\n\n"},
		{"plain", ""},
	} {
		lines := strings.Split(tc.in, "\n")
		last := lines[len(lines)-1]
		m.setEditor(tc.in, len(lines)-1, len([]rune(last)))
		if !m.continueList() {
			if tc.want != "" {
				t.Errorf("%q: not continued", tc.in)
			}
			continue
		}
		if got := m.editor.Value(); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "draft.md")
	if err := os.WriteFile(from, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "taken.md"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := pagerModel{common: &commonModel{cwd: dir}}
	m.currentDocument = markdown{localPath: from, Note: "draft.md"}

	m.rename("taken")
	if m.currentDocument.localPath != from {
		t.Fatal("renamed over an existing file")
	}
	m.rename("notes/idea")
	want := filepath.Join(dir, "notes", "idea.md")
	if m.currentDocument.localPath != want || m.currentDocument.Note != filepath.Join("notes", "idea.md") {
		t.Fatalf("got %q / %q", m.currentDocument.localPath, m.currentDocument.Note)
	}
	if b, err := os.ReadFile(want); err != nil || string(b) != "hi" {
		t.Fatalf("file not moved: %v", err)
	}
}

func TestCursorToTextStart(t *testing.T) {
	m := pagerModel{editor: textarea.New()}
	m.editor.MaxHeight = 0
	m.setEditor("  - item", 0, 7)
	m.cursorToTextStart()
	if c := m.editor.Column(); c != 2 {
		t.Errorf("column %d, want 2", c)
	}
}

func TestWatcherFollowsRename(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	from := filepath.Join(dir, "draft.md")
	if err := os.WriteFile(from, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newPagerModel(&commonModel{cwd: dir})
	m.currentDocument = markdown{localPath: from, Note: "draft.md"}

	// The watcher runs on a copy of the model, as it does in the app.
	got := make(chan tea.Msg, 1)
	w := m
	go func() { got <- w.watchFile() }()
	time.Sleep(50 * time.Millisecond)

	m.rename("notes/idea")
	if err := os.WriteFile(m.currentDocument.localPath, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-got:
		if _, ok := msg.(reloadMsg); !ok {
			t.Fatalf("got %T, want reloadMsg", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no reload after writing the renamed file")
	}
}
