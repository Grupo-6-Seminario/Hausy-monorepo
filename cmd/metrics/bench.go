package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// benchmark is one benchmark's runs from go test -bench output.
type benchmark struct {
	name                             string
	nsPerOp, bytesPerOp, allocsPerOp []float64
}

// runBenchmarks runs every benchmark of the measured packages count times
// and keeps go test's output verbatim, the format benchstat reads.
func runBenchmarks(dir, out string, packages []string, count int) ([]byte, error) {
	args := append([]string{"test", "-run", "^$", "-bench", ".", "-benchmem", "-count", strconv.Itoa(count)}, packages...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go test -bench: %w: %s%s", err, output, stderr.String())
	}
	return output, os.WriteFile(out, output, 0o644)
}

// parseBenchmarks reads go test -bench output, naming each benchmark after
// its package's short name, in the order they ran. cpu is the machine line.
func parseBenchmarks(output []byte, module string) (benchmarks []*benchmark, cpu string) {
	byName := map[string]*benchmark{}
	pkg := ""
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if value, ok := strings.CutPrefix(line, "pkg: "); ok {
			pkg = short(strings.TrimPrefix(strings.TrimPrefix(value, module), "/"))
			continue
		}
		if value, ok := strings.CutPrefix(line, "cpu: "); ok {
			cpu = value
			continue
		}
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "Benchmark") || len(fields) < 4 {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(fields[0], "Benchmark"), "-")
		name = pkg + "." + name
		b := byName[name]
		if b == nil {
			b = &benchmark{name: name}
			byName[name] = b
			benchmarks = append(benchmarks, b)
		}
		for i := 2; i+1 < len(fields); i += 2 {
			value, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				continue
			}
			switch fields[i+1] {
			case "ns/op":
				b.nsPerOp = append(b.nsPerOp, value)
			case "B/op":
				b.bytesPerOp = append(b.bytesPerOp, value)
			case "allocs/op":
				b.allocsPerOp = append(b.allocsPerOp, value)
			}
		}
	}
	return benchmarks, cpu
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(values))
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

func duration(ns float64) string {
	switch {
	case ns >= 1e9:
		return strconv.FormatFloat(ns/1e9, 'f', 2, 64) + " s"
	case ns >= 1e6:
		return strconv.FormatFloat(ns/1e6, 'f', 2, 64) + " ms"
	case ns >= 1e3:
		return strconv.FormatFloat(ns/1e3, 'f', 2, 64) + " µs"
	}
	return strconv.FormatFloat(ns, 'f', 1, 64) + " ns"
}

func size(bytes float64) string {
	switch {
	case bytes >= 1<<20:
		return strconv.FormatFloat(bytes/(1<<20), 'f', 2, 64) + " MiB"
	case bytes >= 1<<10:
		return strconv.FormatFloat(bytes/(1<<10), 'f', 1, 64) + " KiB"
	}
	return strconv.FormatFloat(bytes, 'f', 0, 64) + " B"
}

func benchmarkSection(benchmarks []*benchmark, cpu string) string {
	var b strings.Builder
	b.WriteString("\n## Benchmarks\n\n")
	fmt.Fprintf(&b, "Medians of each benchmark's runs on %s. Raw runs are in `bench.txt`; compare two snapshots with benchstat, on the same machine.\n\n", cpu)
	b.WriteString("| Benchmark | Time/op | Memory/op | Allocs/op | Runs |\n| --- | ---: | ---: | ---: | ---: |\n")
	for _, bench := range benchmarks {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %d |\n", bench.name, duration(median(bench.nsPerOp)), size(median(bench.bytesPerOp)),
			strconv.FormatFloat(median(bench.allocsPerOp), 'f', 0, 64), len(bench.nsPerOp))
	}
	return b.String()
}
