//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxLOC      = 800
	maxLOCProto = 900 // proto generated files exempt, but .proto source capped
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	violations := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// proto generated files are LOC-exempt
		if strings.Contains(path, "proto/") && strings.HasSuffix(path, ".pb.go") {
			return nil
		}

		violations += checkFile(path)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
		os.Exit(1)
	}

	if violations > 0 {
		fmt.Fprintf(os.Stderr, "\n%d discipline violation(s) found\n", violations)
		os.Exit(1)
	}
	fmt.Println("discipline check passed")
}

func checkFile(path string) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", path, err)
		return 1
	}
	defer f.Close()

	violations := 0
	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// R2: no ignored errors — "_ = something(" pattern
		if strings.HasPrefix(trimmed, "_ = ") && strings.Contains(trimmed, "(") {
			fmt.Fprintf(os.Stderr, "R2 %s:%d — ignored error: %s\n", path, lineNum, trimmed)
			violations++
		}

		// R2: empty error blocks — if err != nil { }
		if trimmed == "if err != nil {}" || trimmed == "if err != nil { }" {
			fmt.Fprintf(os.Stderr, "R2 %s:%d — empty error block\n", path, lineNum)
			violations++
		}
	}

	// R1: LOC check
	if lineNum > maxLOC {
		fmt.Fprintf(os.Stderr, "R1 %s — %d lines (max %d)\n", path, lineNum, maxLOC)
		violations++
	}

	return violations
}
