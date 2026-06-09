package tools

import "strings"

// UnifiedDiff returns a line-oriented diff between before and after. Each output
// line is prefixed with '+' (added), '-' (removed), or ' ' (unchanged context),
// computed from the longest common subsequence of lines. A created file
// (before == "") yields all-'+' lines. path is accepted for callers that want to
// associate the diff with a file; the body is what the zeroline renderer parses.
func UnifiedDiff(path, before, after string) string {
	a := diffLines(before)
	b := diffLines(after)
	m, n := len(a), len(b)

	// lcs[i][j] = length of the LCS of a[i:] and b[j:].
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out []string
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case a[i] == b[j]:
			out = append(out, " "+a[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, "-"+a[i])
			i++
		default:
			out = append(out, "+"+b[j])
			j++
		}
	}
	for ; i < m; i++ {
		out = append(out, "-"+a[i])
	}
	for ; j < n; j++ {
		out = append(out, "+"+b[j])
	}
	return strings.Join(out, "\n")
}

// DiffStat counts the added and removed lines in a UnifiedDiff body.
func DiffStat(diff string) (add, del int) {
	if diff == "" {
		return 0, 0
	}
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "+"):
			add++
		case strings.HasPrefix(ln, "-"):
			del++
		}
	}
	return add, del
}

// diffLines splits content into lines, dropping a single trailing newline so a
// file that ends in "\n" doesn't produce a spurious trailing empty line.
func diffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
