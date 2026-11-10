package gitops

import (
	"fmt"
	"strings"

	"dockerpilot/internal/secrets"
)

// Diff returns a compact unified diff. Empty when the texts match.
func Diff(oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a := strings.Split(oldText, "\n")
	b := strings.Split(newText, "\n")
	if len(a) > 400 || len(b) > 400 {
		return secrets.Redact(fmt.Sprintf("diff grande (%d → %d linhas); revise o arquivo no repositório", len(a), len(b)))
	}
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var bld strings.Builder
	bld.WriteString("--- aplicado\n+++ remoto\n")
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			fmt.Fprintf(&bld, "-%s\n", a[i])
			i++
		} else {
			fmt.Fprintf(&bld, "+%s\n", b[j])
			j++
		}
	}
	for ; i < n; i++ {
		fmt.Fprintf(&bld, "-%s\n", a[i])
	}
	for ; j < m; j++ {
		fmt.Fprintf(&bld, "+%s\n", b[j])
	}
	return secrets.Redact(bld.String())
}
