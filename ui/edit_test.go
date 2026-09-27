package ui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
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
