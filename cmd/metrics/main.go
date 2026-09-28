// Command metrics measures the module's package design and code complexity
// and writes a snapshot that later snapshots are compared with:
//
//	go run ./cmd/metrics -out docs/metrics/2026-09-28
//
// Per package it reports Robert C. Martin's abstractness, instability and
// distance from the main sequence, counting coupling only between the
// module's own non-test packages. Per function it reports cyclomatic
// complexity, counted the way gocyclo counts it. docs/metrics/README.md
// explains each number.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("metrics", flag.ContinueOnError)
	dir := flags.String("dir", ".", "module to measure")
	out := flags.String("out", "", "snapshot directory to write")
	exclude := flags.String("exclude", "experiments,cmd/metrics", "comma-separated package prefixes left out of the measurement")
	bench := flags.Bool("bench", false, "also run the measured packages' benchmarks into bench.txt")
	count := flags.Int("count", 10, "runs of each benchmark with -bench")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required")
	}
	module, packages, err := measure(*dir, strings.Split(*exclude, ","))
	if err != nil {
		return err
	}
	functions := byComplexity(packages)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	if err := writePackages(filepath.Join(*out, "packages.csv"), packages); err != nil {
		return err
	}
	if err := writeFunctions(filepath.Join(*out, "functions.csv"), functions); err != nil {
		return err
	}
	for file, chart := range map[string]string{
		"main-sequence.svg": mainSequenceChart(packages),
		"distance.svg":      distanceChart(packages),
		"complexity.svg":    complexityChart(functions, hotspots),
	} {
		if err := os.WriteFile(filepath.Join(*out, file), []byte(chart), 0o644); err != nil {
			return err
		}
	}
	benchmarks := ""
	if *bench {
		importPaths := make([]string, len(packages))
		for n, p := range packages {
			importPaths[n] = strings.TrimSuffix(module+"/"+p.name, "/.")
		}
		output, err := runBenchmarks(*dir, filepath.Join(*out, "bench.txt"), importPaths, *count)
		if err != nil {
			return err
		}
		benchmarks = benchmarkSection(parseBenchmarks(output, module))
	}
	return writeReport(filepath.Join(*out, "REPORT.md"), module, revision(*dir), packages, functions, benchmarks)
}

type pkg struct {
	name       string
	types      int
	interfaces int
	afferent   int
	efferent   int
	thirdParty int
	functions  []function
}

type function struct {
	pkg        string
	name       string
	position   string
	complexity int
}

func (p pkg) abstractness() float64 {
	if p.types == 0 {
		return 0
	}
	return float64(p.interfaces) / float64(p.types)
}

func (p pkg) instability() float64 {
	if p.afferent+p.efferent == 0 {
		return 0
	}
	return float64(p.efferent) / float64(p.afferent+p.efferent)
}

func (p pkg) distance() float64 {
	return math.Abs(p.abstractness() + p.instability() - 1)
}

func (p pkg) maxComplexity() int {
	highest := 0
	for _, f := range p.functions {
		highest = max(highest, f.complexity)
	}
	return highest
}

func (p pkg) meanComplexity() float64 {
	if len(p.functions) == 0 {
		return 0
	}
	total := 0
	for _, f := range p.functions {
		total += f.complexity
	}
	return float64(total) / float64(len(p.functions))
}

type listed struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Imports    []string
	Module     struct{ Path, Dir string }
}

// measure lists the module's packages with the go command, so build
// constraints decide which files count, and parses their non-test files.
func measure(dir string, exclude []string) (string, []pkg, error) {
	cmd := exec.Command("go", "list", "-json=ImportPath,Dir,GoFiles,Imports,Module", "./...")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("go list: %w: %s", err, stderr.String())
	}
	var all []listed
	for decoder := json.NewDecoder(bytes.NewReader(output)); ; {
		var p listed
		if err := decoder.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return "", nil, err
		}
		all = append(all, p)
	}
	if len(all) == 0 {
		return "", nil, fmt.Errorf("no packages in %s", dir)
	}
	module := all[0].Module

	name := func(importPath string) string {
		if importPath == module.Path {
			return "."
		}
		return strings.TrimPrefix(importPath, module.Path+"/")
	}
	inModule := func(importPath string) bool {
		return importPath == module.Path || strings.HasPrefix(importPath, module.Path+"/")
	}
	excluded := func(importPath string) bool {
		rel := name(importPath)
		for _, prefix := range exclude {
			if prefix != "" && (rel == prefix || strings.HasPrefix(rel, prefix+"/")) {
				return true
			}
		}
		return false
	}

	measured := map[string]*pkg{}
	var order []string
	for _, p := range all {
		if !excluded(p.ImportPath) {
			measured[p.ImportPath] = &pkg{name: name(p.ImportPath)}
			order = append(order, p.ImportPath)
		}
	}
	for _, p := range all {
		m := measured[p.ImportPath]
		if m == nil {
			continue
		}
		for _, imported := range p.Imports {
			switch {
			case measured[imported] != nil:
				m.efferent++
				measured[imported].afferent++
			case !inModule(imported) && !standard(imported):
				m.thirdParty++
			}
		}
		if err := parse(m, p, module.Dir); err != nil {
			return "", nil, err
		}
	}

	packages := make([]pkg, 0, len(order))
	for _, importPath := range order {
		packages = append(packages, *measured[importPath])
	}
	slices.SortFunc(packages, func(a, b pkg) int { return strings.Compare(a.name, b.name) })
	return module.Path, packages, nil
}

// standard reports whether an import path belongs to the standard library,
// whose first element never contains a dot.
func standard(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

func parse(m *pkg, p listed, moduleDir string) error {
	fset := token.NewFileSet()
	for _, file := range p.GoFiles {
		syntax, err := parser.ParseFile(fset, filepath.Join(p.Dir, file), nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range syntax.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || typeSpec.Assign != token.NoPos {
						continue
					}
					m.types++
					if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						m.interfaces++
					}
				}
			case *ast.FuncDecl:
				position := fset.Position(decl.Pos())
				rel, err := filepath.Rel(moduleDir, position.Filename)
				if err != nil {
					return err
				}
				m.functions = append(m.functions, function{
					pkg:        m.name,
					name:       funcName(decl),
					position:   filepath.ToSlash(rel) + ":" + strconv.Itoa(position.Line),
					complexity: complexity(decl),
				})
			}
		}
	}
	return nil
}

// funcName names methods the way gocyclo does: (*T).M or (T).M.
func funcName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	pointer := ""
	if star, ok := receiver.(*ast.StarExpr); ok {
		pointer, receiver = "*", star.X
	}
	switch generic := receiver.(type) {
	case *ast.IndexExpr:
		receiver = generic.X
	case *ast.IndexListExpr:
		receiver = generic.X
	}
	typeName := "?"
	if ident, ok := receiver.(*ast.Ident); ok {
		typeName = ident.Name
	}
	return "(" + pointer + typeName + ")." + decl.Name.Name
}

// complexity is McCabe's cyclomatic complexity as gocyclo counts it: one,
// plus one per if, for, range, non-default case, non-default select case,
// && and ||. Function literals count toward the function that holds them.
func complexity(fn *ast.FuncDecl) int {
	count := 1
	ast.Inspect(fn, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			count++
		case *ast.CaseClause:
			if node.List != nil {
				count++
			}
		case *ast.CommClause:
			if node.Comm != nil {
				count++
			}
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}

func writePackages(path string, packages []pkg) error {
	rows := [][]string{{"package", "types", "interfaces", "abstractness", "afferent", "efferent", "instability", "distance", "third_party", "functions", "max_complexity", "mean_complexity"}}
	for _, p := range packages {
		rows = append(rows, []string{
			p.name, strconv.Itoa(p.types), strconv.Itoa(p.interfaces), fixed(p.abstractness()),
			strconv.Itoa(p.afferent), strconv.Itoa(p.efferent), fixed(p.instability()), fixed(p.distance()),
			strconv.Itoa(p.thirdParty), strconv.Itoa(len(p.functions)), strconv.Itoa(p.maxComplexity()), fixed(p.meanComplexity()),
		})
	}
	return writeCSV(path, rows)
}

// byComplexity lists every function, most complex first.
func byComplexity(packages []pkg) []function {
	var all []function
	for _, p := range packages {
		all = append(all, p.functions...)
	}
	slices.SortStableFunc(all, func(a, b function) int {
		if a.complexity != b.complexity {
			return b.complexity - a.complexity
		}
		if a.pkg != b.pkg {
			return strings.Compare(a.pkg, b.pkg)
		}
		return strings.Compare(a.name, b.name)
	})
	return all
}

func writeFunctions(path string, functions []function) error {
	rows := [][]string{{"package", "function", "complexity", "position"}}
	for _, f := range functions {
		rows = append(rows, []string{f.pkg, f.name, strconv.Itoa(f.complexity), f.position})
	}
	return writeCSV(path, rows)
}

func fixed(value float64) string { return strconv.FormatFloat(value, 'f', 2, 64) }

func writeCSV(path string, rows [][]string) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
