package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxLength = 58
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func readLines(path string) []string {
	srcBytes, err := os.ReadFile(path)
	check(err)
	return strings.Split(string(srcBytes), "\n")
}

func isDir(path string) bool {
	fileStat, _ := os.Stat(path)
	return fileStat.IsDir()
}

var commentPat = regexp.MustCompile("\\s*\\/\\/")

func main() {
	sourcePaths, err := filepath.Glob("./examples/**")
	check(err)
	fmt.Printf("found %d files\n", len(sourcePaths))
	foundLongFile := false
	for _, sourcePath := range sourcePaths {
		if strings.HasSuffix(sourcePath, ".md") {
			continue
		}

		foundLongLine := false
		if !isDir(sourcePath) {
			lines := readLines(sourcePath)
			for i, line := range lines {
				// Convert tabs to spaces before measuring, so we get an accurate measure
				// of how long the output will end up being.
				line := strings.Replace(line, "\t", "    ", -1)
				if !foundLongLine && !commentPat.MatchString(line) && (utf8.RuneCountInString(line) > maxLength) {
					fmt.Printf("measure: %s:%d\n", sourcePath, i+1)
					foundLongLine = true
					foundLongFile = true
				}
			}
		}
	}
	if foundLongFile {
		os.Exit(1)
	}
}
