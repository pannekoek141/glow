package ui

import (
	"reflect"
	"strings"
	"testing"
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
