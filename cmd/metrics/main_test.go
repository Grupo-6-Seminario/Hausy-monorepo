package main

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The fixture module testdata/shop is a worked example; every expected value
// below is counted by hand from its source, not recomputed.
//
//	store:    4 types, 2 of them interfaces        A = 2/4 = 0.5
//	          imported by pricing and cmd/shop     Ca = 2, Ce = 0, I = 0
//	          D = |0.5 + 0 - 1| = 0.5
//	pricing:  1 struct                             A = 0
//	          imported by cmd/shop, imports store  Ca = 1, Ce = 1, I = 0.5
//	          D = |0 + 0.5 - 1| = 0.5
//	cmd/shop: no types                             A = 0
//	          imports pricing and store            Ca = 0, Ce = 2, I = 1, D = 0
//	          imports example.com/audit            one third-party package
//
// store_test.go imports pricing; test files are not coupling, so pricing
// still has one dependent.
func TestPackageMetricsOfAWorkedExample(t *testing.T) {
	out := t.TempDir()
	if err := run([]string{"-dir", "testdata/shop", "-out", out}); err != nil {
		t.Fatal(err)
	}
	want := `package,types,interfaces,abstractness,afferent,efferent,instability,distance,third_party,functions,max_complexity,mean_complexity
cmd/shop,0,0,0.00,0,2,1.00,0.00,1,1,2,2.00
pricing,1,0,0.00,1,1,0.50,0.50,0,1,2,2.00
store,4,2,0.50,2,0,0.00,0.50,0,2,8,5.00
`
	if got := readFile(t, filepath.Join(out, "packages.csv")); got != want {
		t.Fatalf("packages.csv:\n%s\nwant:\n%s", got, want)
	}
}

// Discount in testdata/shop/store/store.go has eight paths: one, plus if,
// ||, range, two non-default cases, if and &&. The default case and the
// function in store_test.go do not count.
func TestFunctionComplexityOfAWorkedExample(t *testing.T) {
	out := t.TempDir()
	if err := run([]string{"-dir", "testdata/shop", "-out", out}); err != nil {
		t.Fatal(err)
	}
	want := `package,function,complexity,position
store,Discount,8,store/store.go:19
cmd/shop,main,2,cmd/shop/main.go:13
pricing,Price,2,pricing/pricing.go:11
store,(*Item).Cost,2,store/store.go:39
`
	if got := readFile(t, filepath.Join(out, "functions.csv")); got != want {
		t.Fatalf("functions.csv:\n%s\nwant:\n%s", got, want)
	}
}

// REPORT.md is what a reader opens: every chart it shows must be in the
// snapshot, be well-formed SVG, and the main sequence must name every package.
func TestReportShowsChartsFromTheSnapshot(t *testing.T) {
	out := t.TempDir()
	if err := run([]string{"-dir", "testdata/shop", "-out", out}); err != nil {
		t.Fatal(err)
	}
	report := readFile(t, filepath.Join(out, "REPORT.md"))
	images := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`).FindAllStringSubmatch(report, -1)
	if len(images) != 3 {
		t.Fatalf("want the main sequence, distance and complexity charts, got %d images:\n%s", len(images), report)
	}
	for _, image := range images {
		decoder := xml.NewDecoder(strings.NewReader(readFile(t, filepath.Join(out, image[1]))))
		for {
			if _, err := decoder.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed: %v", image[1], err)
			}
		}
	}
	mainSequence := readFile(t, filepath.Join(out, "main-sequence.svg"))
	for _, name := range []string{"store", "pricing", "cmd/shop"} {
		if !strings.Contains(mainSequence, ">"+name+"<") {
			t.Errorf("main sequence does not label %s", name)
		}
	}
}

// A snapshot is committed and diffed against later ones, so measuring the
// same code twice must write the same bytes.
func TestSnapshotsOfTheSameCodeAreIdentical(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, out := range []string{first, second} {
		if err := run([]string{"-dir", "testdata/shop", "-out", out}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if readFile(t, filepath.Join(first, entry.Name())) != readFile(t, filepath.Join(second, entry.Name())) {
			t.Errorf("%s differs between two runs", entry.Name())
		}
	}
}

// With -bench the snapshot keeps go test's raw output, which benchstat
// compares across snapshots, and the report shows each benchmark.
func TestBenchmarksJoinTheSnapshot(t *testing.T) {
	out := t.TempDir()
	if err := run([]string{"-dir", "testdata/shop", "-out", out, "-bench", "-count", "2"}); err != nil {
		t.Fatal(err)
	}
	if runs := strings.Count(readFile(t, filepath.Join(out, "bench.txt")), "BenchmarkDiscount-"); runs != 2 {
		t.Fatalf("bench.txt holds %d runs of BenchmarkDiscount, want 2", runs)
	}
	if row := regexp.MustCompile("(?m)^\\| `store.Discount` \\| [0-9.]+ ns \\| 0 B \\| 0 \\| 2 \\|$"); !row.MatchString(readFile(t, filepath.Join(out, "REPORT.md"))) {
		t.Fatalf("REPORT.md has no row for store.Discount matching %s", row)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
