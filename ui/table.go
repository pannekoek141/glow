package ui

import (
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Glamour draws tables without an outer border. boxTables adds one, so a
// table looks like a closed box with a line under the header.

var tableRule = regexp.MustCompile(`^( *)(─+(?:┼─+)+) *$`)

func boxTables(lines []string) []string {
	for i := 0; i < len(lines); i++ {
		m := tableRule.FindStringSubmatch(ansi.Strip(lines[i]))
		if m == nil || len(m[1]) == 0 {
			continue
		}
		// The border replaces the space before and the last column of the
		// table (cell padding), so the width stays the same.
		start, rule := len(m[1]), strings.TrimSuffix(m[2], "─")
		end := start + ansi.StringWidth(rule)
		var cols []int
		c := start
		for _, r := range rule {
			if r == '┼' {
				cols = append(cols, c)
			}
			c++
		}
		isRow := func(l string) bool {
			p := ansi.Strip(l)
			for _, c := range cols {
				if ansi.Strip(ansi.Cut(p, c, c+1)) != "│" {
					return false
				}
			}
			return true
		}
		top, bottom := i, i+1
		for top > 0 && isRow(lines[top-1]) {
			top--
		}
		for bottom < len(lines) && isRow(lines[bottom]) {
			bottom++
		}

		pad := strings.Repeat(" ", start-1)
		edge := func(l, mid, r string) string { return pad + l + strings.ReplaceAll(rule, "┼", mid) + r }
		out := []string{edge("┌", "┬", "┐")}
		for j := top; j < bottom; j++ {
			if j == i {
				out = append(out, edge("├", "┼", "┤"))
				continue
			}
			out = append(out, pad+"│"+ansi.Cut(lines[j], start, end)+"│")
		}
		out = append(out, edge("└", "┴", "┘"))
		lines = slices.Concat(lines[:top], out, lines[bottom:])
		i = top + len(out) - 1
	}
	return lines
}
