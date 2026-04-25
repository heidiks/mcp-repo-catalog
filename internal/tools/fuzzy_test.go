package tools

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"auth", "atuh", 2},
		{"gateway", "gatewey", 1},
	}

	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSplitNameTokens(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"foo", []string{"foo"}},
		{"foo-bar", []string{"foo", "bar"}},
		{"foo_bar_baz", []string{"foo", "bar", "baz"}},
		{"a-b_c", []string{"a", "b", "c"}},
		{"-leading", []string{"leading"}},
		{"trailing-", []string{"trailing"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitNameTokens(tt.name)
		if len(got) != len(tt.want) {
			t.Errorf("splitNameTokens(%q) = %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitNameTokens(%q)[%d] = %q, want %q", tt.name, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFuzzyThreshold(t *testing.T) {
	tests := []struct {
		queryLen int
		want     int
	}{
		{1, 1},
		{3, 1},
		{6, 2},
		{9, 3},
		{12, 4},
		{30, 4},
	}

	for _, tt := range tests {
		got := fuzzyThreshold(tt.queryLen)
		if got != tt.want {
			t.Errorf("fuzzyThreshold(%d) = %d, want %d", tt.queryLen, got, tt.want)
		}
	}
}
