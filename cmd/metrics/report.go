package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// hotspots is how many of the most complex functions the report ranks.
const hotspots = 15

func writeReport(path, module, revision string, packages []pkg, functions []function, benchmarks string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Metrics snapshot\n\n`%s` at `%s`, measured by `go run ./cmd/metrics`.\n", module, revision)
	b.WriteString("What each number means and how to compare snapshots: `docs/metrics/README.md`.\n\n")

	ranked := slices.Clone(packages)
	slices.SortStableFunc(ranked, func(a, b pkg) int { return compareDesc(a.distance(), b.distance()) })
	totalDistance, inZones := 0.0, []string{}
	for _, p := range ranked {
		totalDistance += p.distance()
		if p.distance() > 0.7 {
			inZones = append(inZones, "`"+p.name+"`")
		}
	}
	complexities := make([]int, len(functions))
	over, total := 0, 0
	for n, f := range functions {
		complexities[n] = f.complexity
		total += f.complexity
		if f.complexity > 10 {
			over++
		}
	}
	slices.Sort(complexities)

	b.WriteString("| Summary | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Packages | %d |\n", len(packages))
	fmt.Fprintf(&b, "| Mean distance from the main sequence | %s |\n", fixed(totalDistance/float64(max(1, len(packages)))))
	fmt.Fprintf(&b, "| Packages with D > 0.7 | %d |\n", len(inZones))
	fmt.Fprintf(&b, "| Functions | %d |\n", len(functions))
	if len(functions) > 0 {
		fmt.Fprintf(&b, "| Mean cyclomatic complexity | %s |\n", fixed(float64(total)/float64(len(functions))))
		fmt.Fprintf(&b, "| Median cyclomatic complexity | %d |\n", complexities[len(complexities)/2])
		fmt.Fprintf(&b, "| Functions over 10 | %d (%s%%) |\n", over, fixed(100*float64(over)/float64(len(functions))))
		fmt.Fprintf(&b, "| Highest | %d, `%s.%s` |\n", functions[0].complexity, short(functions[0].pkg), functions[0].name)
	}

	b.WriteString("\n## Package design: abstractness, instability, distance\n\n")
	b.WriteString("![Abstractness vs instability](main-sequence.svg)\n\n![Distance from the main sequence](distance.svg)\n\n")
	if len(inZones) > 0 {
		fmt.Fprintf(&b, "Far from the main sequence (D > 0.7): %s.\n\n", strings.Join(inZones, ", "))
	}
	b.WriteString("| Package | A | I | D | Ca | Ce | Types | Interfaces | Third-party imports |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, p := range ranked {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %d | %d | %d | %d | %d |\n", p.name, fixed(p.abstractness()), fixed(p.instability()), fixed(p.distance()),
			p.afferent, p.efferent, p.types, p.interfaces, p.thirdParty)
	}

	b.WriteString("\n## Cyclomatic complexity\n\n")
	b.WriteString("| Band | Functions | Share |\n| --- | ---: | ---: |\n")
	for _, band := range []struct {
		name     string
		low, top int
	}{{"1–10 simple", 1, 10}, {"11–20 moderate", 11, 20}, {"21–50 complex", 21, 50}, {"over 50 untestable", 51, int(^uint(0) >> 1)}} {
		count := 0
		for _, c := range complexities {
			if c >= band.low && c <= band.top {
				count++
			}
		}
		fmt.Fprintf(&b, "| %s | %d | %s%% |\n", band.name, count, fixed(100*float64(count)/float64(max(1, len(functions)))))
	}
	b.WriteString("\n![Most complex functions](complexity.svg)\n\n")
	b.WriteString("| Function | Complexity | Position |\n| --- | ---: | --- |\n")
	for _, f := range functions[:min(hotspots, len(functions))] {
		fmt.Fprintf(&b, "| `%s.%s` | %d | `%s` |\n", short(f.pkg), f.name, f.complexity, f.position)
	}

	byMax := slices.Clone(packages)
	slices.SortStableFunc(byMax, func(a, b pkg) int { return b.maxComplexity() - a.maxComplexity() })
	b.WriteString("\n| Package | Functions | Mean | Max |\n| --- | ---: | ---: | ---: |\n")
	for _, p := range byMax {
		fmt.Fprintf(&b, "| `%s` | %d | %s | %d |\n", p.name, len(p.functions), fixed(p.meanComplexity()), p.maxComplexity())
	}
	b.WriteString("\nEvery function is in `functions.csv`; every package in `packages.csv`.\n")
	b.WriteString(benchmarks)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// revision names the measured commit, marked dirty when tracked files have
// uncommitted changes.
func revision(dir string) string {
	cmd := exec.Command("git", "describe", "--always", "--dirty", "--abbrev=7")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "unknown revision"
	}
	return strings.TrimSpace(string(output))
}
