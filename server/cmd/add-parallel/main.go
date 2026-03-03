package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	topLevelTestRe = regexp.MustCompile(`^func Test\w+\(t \*testing\.T\) \{$`)
	forRangeRe     = regexp.MustCompile(`^(\s+)for _,\s+(\w+)\s+:= range`)
	tRunVarRe      = regexp.MustCompile(`^(\s+)t\.Run\((\w+)\.\w+,\s*func\(t \*testing\.T\) \{$`)
	tRunLitRe      = regexp.MustCompile(`^(\s+)t\.Run\("[^"]*",\s*func\(t \*testing\.T\) \{$`)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: add-parallel <file>...")
		os.Exit(1)
	}
	for _, path := range os.Args[1:] {
		if err := processFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("OK: %s\n", path)
	}
}

func nextNonEmptyTrimmed(lines []string, i int) string {
	for j := i; j < len(lines); j++ {
		s := strings.TrimSpace(lines[j])
		if s != "" {
			return s
		}
	}
	return ""
}

func processFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	loopVar := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Top-level test function: add t.Parallel() after opening brace
		if topLevelTestRe.MatchString(line) {
			out = append(out, line)
			next := nextNonEmptyTrimmed(lines, i+1)
			if next != "t.Parallel()" {
				out = append(out, "\tt.Parallel()")
				out = append(out, "")
			}
			loopVar = ""
			continue
		}

		// for _, xx := range: track loop variable
		if m := forRangeRe.FindStringSubmatch(line); m != nil {
			loopVar = m[2]
			out = append(out, line)
			continue
		}

		// t.Run with loop variable: add capture + t.Parallel()
		if m := tRunVarRe.FindStringSubmatch(line); m != nil && loopVar != "" && m[2] == loopVar {
			indent := m[1]
			captureLine := indent + loopVar + " := " + loopVar
			// Check if previous non-empty line already has the capture
			prevTrimmed := ""
			for j := len(out) - 1; j >= 0; j-- {
				s := strings.TrimSpace(out[j])
				if s != "" {
					prevTrimmed = s
					break
				}
			}
			if prevTrimmed != loopVar+" := "+loopVar {
				out = append(out, captureLine)
			}
			out = append(out, line)
			next := nextNonEmptyTrimmed(lines, i+1)
			if next != "t.Parallel()" {
				out = append(out, indent+"\tt.Parallel()")
				out = append(out, "")
			}
			continue
		}

		// t.Run with literal string: just add t.Parallel()
		if m := tRunLitRe.FindStringSubmatch(line); m != nil {
			indent := m[1]
			out = append(out, line)
			next := nextNonEmptyTrimmed(lines, i+1)
			if next != "t.Parallel()" {
				out = append(out, indent+"\tt.Parallel()")
				out = append(out, "")
			}
			continue
		}

		out = append(out, line)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0600)
}
