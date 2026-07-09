package diff

import (
	"fmt"
	"strings"
)

type opType int

const (
	opEqual opType = iota
	opInsert
	opDelete
)

type diffOp struct {
	typ   opType
	line  string
	oldNo int
	newNo int
}

// UnifiedDiff computes a unified diff between oldText and newText.
// Returns the diff text with standard @@ -oldStart,oldCount +newStart,newCount @@ hunk headers.
func UnifiedDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	if len(oldLines) == 0 && len(newLines) == 0 {
		return ""
	}

	ops := computeDiff(oldLines, newLines)
	hunks := buildHunks(ops, 3)
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, h := range hunks {
		sb.WriteString(fmt.Sprintf("@@ -%s +%s @@\n", formatRange(h.oldStart, h.oldCount), formatRange(h.newStart, h.newCount)))
		for _, op := range h.ops {
			switch op.typ {
			case opEqual:
				sb.WriteString(" " + op.line + "\n")
			case opInsert:
				sb.WriteString("+" + op.line + "\n")
			case opDelete:
				sb.WriteString("-" + op.line + "\n")
			}
		}
	}
	return sb.String()
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func computeDiff(oldLines, newLines []string) []diffOp {
	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var ops []diffOp
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = prepend(ops, diffOp{typ: opEqual, line: oldLines[i-1], oldNo: i, newNo: j})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = prepend(ops, diffOp{typ: opInsert, line: newLines[j-1], newNo: j})
			j--
		} else {
			ops = prepend(ops, diffOp{typ: opDelete, line: oldLines[i-1], oldNo: i})
			i--
		}
	}
	return ops
}

func prepend(ops []diffOp, op diffOp) []diffOp {
	ops = append(ops, diffOp{})
	copy(ops[1:], ops[:len(ops)-1])
	ops[0] = op
	return ops
}

type hunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	ops      []diffOp
}

func buildHunks(ops []diffOp, context int) []hunk {
	var hunks []hunk
	i := 0
	for i < len(ops) {
		if ops[i].typ == opEqual {
			i++
			continue
		}

		start := i - context
		if start < 0 {
			start = 0
		}

		end := i
		trailing := 0
		for end < len(ops) {
			if ops[end].typ != opEqual {
				trailing = 0
			} else {
				trailing++
				if trailing > context {
					break
				}
			}
			end++
		}
		if trailing > context {
			end--
		}

		h := hunk{ops: ops[start:end]}
		for _, op := range h.ops {
			if op.oldNo > 0 {
				if h.oldStart == 0 || op.oldNo < h.oldStart {
					h.oldStart = op.oldNo
				}
				h.oldCount++
			}
			if op.newNo > 0 {
				if h.newStart == 0 || op.newNo < h.newStart {
					h.newStart = op.newNo
				}
				h.newCount++
			}
		}
		hunks = append(hunks, h)
		i = end
	}
	return hunks
}

func formatRange(start, count int) string {
	if count <= 1 {
		if count == 0 {
			start = 0
		}
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
