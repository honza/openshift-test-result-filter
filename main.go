// openshift-test-result-filter
// Copyright (C) 2021 Honza Pokorny <honza@pokorny.ca>

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joshdk/go-junit"
)

var contextPattern = regexp.MustCompile(`^\[(?P<label>[\w\-\.]+)\]`)
var tagPattern = regexp.MustCompile(`\[(?P<label>[\w\-\.:/]+)\]`)

type TestCase struct {
	Name string

	// Name without context and tags
	SimpleName string

	Context string
	Tags    []string
	Test    junit.Test
}

func (tc TestCase) IsSkipped() bool {
	return tc.Test.Status == "skipped"
}

func (tc TestCase) IsPassed() bool {
	return tc.Test.Status == "passed"
}

func (tc TestCase) IsFailed() bool {
	return tc.Test.Status == "failed"
}

type SourceLocation struct {
	Path       string
	LineNumber int
}

func (sl SourceLocation) PrettyString() string {
	return "Test source code location: " + sl.GitHubLink()
}

func (sl SourceLocation) GitHubLink() string {
	return fmt.Sprintf("https://github.com/openshift/origin/blob/master%s#L%d", sl.Path, sl.LineNumber)
}

func GetSimpleName(name string, context string, tags []string) string {
	for _, tag := range tags {
		name = strings.ReplaceAll(name, "["+tag+"]", "")
	}

	name = strings.ReplaceAll(name, "["+context+"] ", "")
	return name
}

func LoadData(filename string) ([]TestCase, error) {
	var entries []TestCase

	suites, err := junit.IngestFile(filename)

	if err != nil {
		return entries, err
	}

	for _, suite := range suites {

		for _, test := range suite.Tests {
			context := ParseContext(test.Name)
			tags := ParseTags(test.Name)
			simpleName := GetSimpleName(test.Name, context, tags)

			testCase := TestCase{
				Name:       test.Name,
				SimpleName: simpleName,
				Context:    context,
				Test:       test,
			}
			entries = append(entries, testCase)
		}
	}

	return entries, nil

}

func ParseContext(name string) string {
	matches := contextPattern.FindStringSubmatch(name)

	for i, m := range matches {
		if i != 0 {
			return m
		}
	}

	return ""
}

func ParseTags(name string) []string {
	matches := tagPattern.FindAllStringSubmatch(name, -1)
	tags := []string{}

	for i, submatches := range matches {
		if i != 0 {
			for j, sm := range submatches {
				if j != 0 {
					tags = append(tags, sm)
				}
			}
		}
	}

	return tags

}

type OriginCache map[string]string

func CreateOriginTestCache(originSource string) (OriginCache, error) {
	cache := make(map[string]string)

	err := filepath.WalkDir(originSource, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.Contains(path, "zz_generated") {
			return nil
		}

		contents, err := ioutil.ReadFile(path)

		if err != nil {
			return err
		}

		cache[path] = string(contents)

		return nil
	})

	if err != nil {
		return cache, err
	}

	return cache, nil
}

func FindStringInCache(cache OriginCache, pattern string) (matched bool, path string, lineNumber int, err error) {
	for path, contents := range cache {
		r, err := regexp.Compile(regexp.QuoteMeta(pattern))

		if err != nil {
			return false, "", 0, err
		}

		loc := r.FindStringIndex(contents)

		if len(loc) > 0 {
			above := string(contents[:loc[0]])
			lineCountAbove := len(strings.Split(above, "\n"))

			return true, path, lineCountAbove, nil
		}
	}

	return false, "", 0, nil

}

func FindTestSource(originSource string, cache OriginCache, tc TestCase) (bool, SourceLocation, error) {
	sl := SourceLocation{}

	words := strings.Split(tc.Name, " ")

	for i := len(words); i >= 0; i-- {
		stringToTry := strings.Join(words[:i], " ")

		matched, path, lineNumber, err := FindStringInCache(cache, stringToTry)

		if err != nil {
			return false, sl, err
		}

		if !matched {
			continue
		}

		sl = SourceLocation{
			Path:       strings.ReplaceAll(path, originSource, ""),
			LineNumber: lineNumber,
		}
		break

	}

	return true, sl, nil
}

var tag = flag.String("tag", "", "Tag, e.g. sig-storage")
var result = flag.String("result", "all", "choices: all, skipped, failed, passed")
var filename = flag.String("filename", "", "input junit file")
var showErrors = flag.Bool("show-errors", false, "")
var originTreePath = flag.String("origin-tree-path", "", "")

func main() {
	flag.Parse()

	if *filename == "" {
		fmt.Println("missing input filename")
		os.Exit(1)
	}

	if *originTreePath == "" {
		fmt.Println("missing origin-tree-path")
		os.Exit(1)
	}

	entries, err := LoadData(*filename)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var cache OriginCache

	if *originTreePath != "" {
		cache, err = CreateOriginTestCache(*originTreePath)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	for _, entry := range entries {
		if *tag != "" {
			foundTag := false
			for _, t := range entry.Tags {
				if *tag == t {
					foundTag = true
					break
				}
			}
			if !foundTag {
				continue
			}
		}

		if *result == "skipped" && !entry.IsSkipped() {
			continue
		}

		if *result == "failed" && !entry.IsFailed() {
			continue
		}

		if *result == "passed" && !entry.IsPassed() {
			continue
		}

		fmt.Println(entry.SimpleName)
		fmt.Println("context:", entry.Context)

		if len(entry.Tags) > 0 {
			fmt.Println("tags:")
			for _, tag := range entry.Tags {
				fmt.Println(" -", tag)
			}
		}

		if *originTreePath != "" {

			found, sl, err := FindTestSource(*originTreePath, cache, entry)

			if err != nil {
				fmt.Println("ERR:", err)
			}

			if found {
				fmt.Println(sl.PrettyString())
			} else {
				fmt.Println("Source not found")
			}

		}

		if *result == "failed" || *result == "all" {
			if *showErrors {
				fmt.Println("ERROR:")
				fmt.Println(entry.Test.Error)
			}
		}

		fmt.Println("-")

	}

}
