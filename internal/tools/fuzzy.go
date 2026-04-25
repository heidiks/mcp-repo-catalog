package tools

// levenshtein returns the edit distance between two lowercase strings.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// fuzzyThreshold returns the max edit distance accepted for a query of given length.
// Allow ~30% of query length, with a floor of 1 and ceiling of 4.
// splitNameTokens splits a repo name into tokens by '-' and '_'.
func splitNameTokens(name string) []string {
	var tokens []string
	start := 0
	for i := 0; i < len(name); i++ {
		if name[i] == '-' || name[i] == '_' {
			if i > start {
				tokens = append(tokens, name[start:i])
			}
			start = i + 1
		}
	}
	if start < len(name) {
		tokens = append(tokens, name[start:])
	}
	return tokens
}

func fuzzyThreshold(queryLen int) int {
	t := queryLen / 3
	if t < 1 {
		t = 1
	}
	if t > 4 {
		t = 4
	}
	return t
}
