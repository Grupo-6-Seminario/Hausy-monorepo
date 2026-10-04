package comparison_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	comparison "github.com/Grupo-6-Seminario/proyecto-angus-back/experiments/property-matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func TestComparisonPreservesRealBuyerBaselineAndMandatoryAlternatives(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, item := range cases {
		if item.ID == "mandatory-alternatives" {
			c = item
		}
	}
	report, err := comparison.Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	got := report.Proposed.Branches[0]
	want := []string{"fixture:a", "fixture:b", "fixture:h", "fixture:c", "fixture:d"}
	urls := []string{}
	for _, row := range got.Shown {
		urls = append(urls, row.Listing.URL)
	}
	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("shown=%v", urls)
	}
	if got.HiddenIneligible != 1 || got.ExcludedContradictions != 1 {
		t.Fatalf("counts=%+v", got)
	}
	baselineContainsContradiction := false
	for _, row := range report.Baseline.Listings {
		if row.URL == "fixture:e" {
			baselineContainsContradiction = true
		}
	}
	if !baselineContainsContradiction {
		t.Fatal("real current buyer baseline defect must remain observable")
	}
	if report.BaselinePacket.Intent != "new_search" {
		t.Fatalf("packet=%+v", report.BaselinePacket)
	}
}

func TestFrozenDevelopmentCases(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.Split == "heldout" {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			result, err := comparison.Run(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Correct {
				t.Fatalf("comparison findings=%v", result.Findings)
			}
		})
	}
}

func TestTargetedQuestionPacketAndFallbackContainOnlyTheRequestedListing(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "mandatory-alternatives" {
			c = v
		}
	}
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	packet := record.Packet("fixture:d", "eligibility")
	count := 0
	for _, b := range packet.Branches {
		for _, r := range b.Shown {
			count++
			if r.Listing.URL != "fixture:d" {
				t.Fatalf("unrelated listing %s in targeted packet", r.Listing.URL)
			}
		}
	}
	if count != 1 {
		t.Fatalf("target count=%d", count)
	}
	got := record.Fallback("fixture:d", "eligibility")
	if got != "El aviso no publica requisitos suficientes para saber si podés alquilarla." {
		t.Fatalf("answer=%q", got)
	}
}

func TestFallbackKeepsPeerNumericEvidencePastTheWriterShortlist(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "price-replaces-score" {
			c = v
		}
	}
	extra := c.Candidates[0]
	extra.Listing.URL = "fixture:z"
	extra.Listing.Address = "Ejemplo z"
	price := 950000.0
	extra.Listing.Price.Amount = &price
	c.Candidates = append(c.Candidates, extra)
	c.Answers["fixture:z"] = c.Answers["fixture:a"]
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	got := record.Fallback("", "")
	if strings.Contains(got, "esa opción no") {
		t.Fatalf("published price falsely called missing: %s", got)
	}
}

func TestLoneSupportedPreferenceGetsAFitExplanation(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "stable-tie" {
			c = v
		}
	}
	c.Candidates = c.Candidates[:1]
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	got := record.Fallback("", "")
	if !strings.Contains(got, "el aviso describe la propiedad como luminosa") {
		t.Fatalf("missing lone fit evidence: %s", got)
	}
	if strings.Contains(got, "antes de") {
		t.Fatalf("fabricated comparison: %s", got)
	}
}

func TestSnapshotNoPetsAdmitsSearcherWithoutPets(t *testing.T) {
	url := "https://www.zonaprop.com.ar/propiedades/clasificado/alclapin-departamento-de-2-ambientes-en-alquiler-en-las-60033803.html"
	c := comparison.Case{ID: "no-pets-control", Plan: intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}, Operation: "alquiler"}}}, BranchIDs: []string{"palermo"}, Criteria: [][]matching.FitCriterion{{}}, Qualification: eligibility.Qualification{"guarantee": {"propietaria"}, "pets": {"none"}}, SnapshotURLs: []string{url}, Expected: map[string][]string{"palermo": {url}}}
	fixture := filepath.Join(t.TempDir(), "cases.json")
	data, err := json.Marshal([]comparison.Case{c})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, data, 0600); err != nil {
		t.Fatal(err)
	}
	cases, _, err := comparison.Load("../..", fixture)
	if err != nil {
		t.Fatal(err)
	}
	result, err := comparison.Run(context.Background(), cases[0])
	if err != nil {
		t.Fatal(err)
	}
	if !result.Correct || len(result.Baseline.Listings) != 1 || len(result.Proposed.Branches[0].Shown) != 1 {
		t.Fatalf("no pets was rejected: %+v", result.Proposed.Branches)
	}
	if result.Proposed.Branches[0].Shown[0].Eligibility.State != eligibility.Eligible {
		t.Fatalf("eligibility=%+v", result.Proposed.Branches[0].Shown[0].Eligibility)
	}
}

func TestIncompleteFrozenEvidenceFailsBeforeBecomingProviderUnavailable(t *testing.T) {
	for _, kind := range []string{"missing answer", "bad reference"} {
		t.Run(kind, func(t *testing.T) {
			cases, _, err := comparison.Load("../..", "fixtures/cases.json")
			if err != nil {
				t.Fatal(err)
			}
			var c comparison.Case
			for _, v := range cases {
				if v.ID == "stable-tie" {
					c = v
				}
			}
			if kind == "missing answer" {
				delete(c.Answers["fixture:a"], "natural_light=high")
			} else {
				a := c.Answers["fixture:a"]["natural_light=high"]
				a.EvidenceRefs = []string{"another-listing"}
				c.Answers["fixture:a"]["natural_light=high"] = a
			}
			if _, err := comparison.RunProposed(context.Background(), c); err == nil {
				t.Fatal("invalid frozen input was hidden as unavailable assessment")
			}
		})
	}
}

func TestRetainedRecordSurvivesCallerAndProjectionMutation(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "weighted-net" {
			c = v
		}
	}
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	projected := record.Project()
	projected.Branches[0].Shown[0].Fit.Contributions[0].Points = 99
	*c.Candidates[0].Listing.Price.Amount = 123
	fresh := record.Project().Branches[0].Shown[0]
	if fresh.Fit.Contributions[0].Points != 2 || fresh.Fit.Numerator != 3 || *fresh.Listing.Price.Amount != 900000 {
		t.Fatalf("record corrupted: %+v", fresh)
	}
}

func TestGuaranteeRelaxationCannotReadmitARequiredContradiction(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "mandatory-alternatives" {
			c = v
		}
	}
	c.Answers["fixture:f"]["natural_light=high"] = comparison.RecordedAnswer{Assessment: "contradicted", Status: "evaluated", EvidenceRefs: []string{"description"}}
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Project().Relaxations) != 0 {
		t.Fatalf("false re-admission promise: %+v", record.Project().Relaxations)
	}
}
func TestTargetedQuestionPreservesPublishedDecimal(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "stable-tie" {
			c = v
		}
	}
	area := 25.3
	c.Candidates[0].Listing.TotalAreaM2 = &area
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if got := record.Fallback("fixture:b", "area"); got != "El aviso publica 25.3 m² de superficie total." {
		t.Fatalf("rounded published fact: %s", got)
	}
}

func TestWriterGetsActualOrderingStatementsAndCountsSharedListingsOnce(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"weighted-net", "branch-specific-shared"} {
		var c comparison.Case
		for _, v := range cases {
			if v.ID == id {
				c = v
			}
		}
		record, err := comparison.RunProposed(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		input := record.WriterInput("", "")
		if id == "weighted-net" && input.Listings[0].ReasonText != "Queda antes de #2 porque el aviso describe el departamento como silencioso; en #2, la información del aviso contradice tu preferencia de silencio." {
			t.Fatalf("wrong reason: %s", input.Listings[0].ReasonText)
		}
		if id == "branch-specific-shared" && input.Overview != "Encontré 2 opciones para mostrarte." {
			t.Fatalf("duplicate options count: %s", input.Overview)
		}
		if len(input.Listings) > 3 {
			t.Fatalf("writer received %d listings", len(input.Listings))
		}
	}
}

func TestFrozenChatQualificationMatchesTheCurrentBuyerInputs(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "empty-results" {
			c = v
		}
	}
	c.Candidates = c.Candidates[:1]
	c.Plan.Qualification = eligibility.Qualification{"guarantee": {"caucion"}}
	baseline, _, _, err := comparison.RunBaseline(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Listings) != 1 || len(record.Project().Branches[0].Shown) != 1 {
		t.Fatal("fixed planner qualification was not merged consistently")
	}
	if record.Project().Branches[0].Shown[0].Eligibility.State != eligibility.Eligible {
		t.Fatal("explicit caucion was not assessed")
	}
}

func TestPriorityMustExistInTheOriginalSearchAndPriceSortNeedsCurrency(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"invented priority", "missing sort currency"} {
		t.Run(kind, func(t *testing.T) {
			var c comparison.Case
			for _, v := range cases {
				if v.ID == "weighted-net" {
					c = v
				}
			}
			if kind == "invented priority" {
				c.Question = "Busco luz y silencio."
			} else {
				c.Plan.Sort = "price_asc"
				c.Plan.Branches[0].Currency = ""
				c.Plan.Branches[0].MaxPrice = nil
			}
			if _, err := comparison.RunProposed(context.Background(), c); err == nil {
				t.Fatalf("%s accepted", kind)
			}
		})
	}
}

type selectionClient struct {
	selection string
	calls     int
}

func (s *selectionClient) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	s.calls++
	if s.calls == 1 {
		return &llm.ChatResponse{Content: "Recorded current writer reply"}, nil
	}
	return &llm.ChatResponse{Content: s.selection}, nil
}
func TestClosedWriterRejectsMalformedForeignDroppedDuplicatedOrReorderedBlocks(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "lone-result" {
			c = v
		}
	}
	for _, tc := range []struct{ name, selection, status string }{
		{"valid", `{"block_ids":["overview","property:palermo:1"]}`, "validated_blocks"},
		{"malformed", `un texto inventado`, "fallback"},
		{"foreign", `{"block_ids":["overview","foreign"]}`, "fallback"},
		{"dropped", `{"block_ids":["overview"]}`, "fallback"},
		{"duplicated", `{"block_ids":["overview","overview","property:palermo:1"]}`, "fallback"},
		{"reordered", `{"block_ids":["property:palermo:1","overview"]}`, "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pair, err := comparison.GeneratePair(context.Background(), c, &selectionClient{selection: tc.selection})
			if err != nil {
				t.Fatal(err)
			}
			if pair.Proposed.Status != tc.status {
				t.Fatalf("unsafe selection status=%s", pair.Proposed.Status)
			}
			if !strings.Contains(pair.Proposed.Content, "La información del aviso contradice tu preferencia de silencio.") {
				t.Fatalf("required negative preference lost: %s", pair.Proposed.Content)
			}
		})
	}
}

func TestPriorityReasonLeadsWithWinnerEvidenceAndNamesTheTradeoff(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "weighted-net" {
			c = v
		}
	}
	c.Candidates = c.Candidates[:2]
	c.Question = "priorizo silencio"
	c.Criteria[0][0].Weight = 1
	c.Criteria[0][0].PrioritySource = ""
	c.Criteria[0][0].SourceQuote = ""
	c.Criteria[0][1].Weight = 2
	c.Criteria[0][1].PrioritySource = "priorizo silencio"
	c.Criteria[0][1].SourceQuote = "priorizo silencio"
	c.Answers["fixture:a"]["natural_light=high"] = comparison.RecordedAnswer{Assessment: "insufficient_evidence", Status: "evaluated"}
	c.Answers["fixture:a"]["noise_level=quiet"] = comparison.RecordedAnswer{Assessment: "supported", Status: "evaluated", EvidenceRefs: []string{"description"}}
	c.Answers["fixture:b"]["noise_level=quiet"] = comparison.RecordedAnswer{Assessment: "insufficient_evidence", Status: "evaluated"}
	c.Candidates[0].Listing.Description = "Departamento silencioso."
	c.Candidates[1].Listing.Description = "Departamento luminoso."
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	reason := record.WriterInput("", "").Listings[0].ReasonText
	want := "Queda antes de #2 porque priorizaste silencio y el aviso describe el departamento como silencioso; en #2, el aviso no confirma silencio. A cambio, el aviso no confirma buena luz natural; en #2, el aviso describe la propiedad como luminosa."
	if reason != want {
		t.Fatalf("misleading deciding reason: %s", reason)
	}
}

func TestRequirementOnlyLoneWriterStatesConfirmedFit(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "mandatory-alternatives" {
			c = v
		}
	}
	c.Candidates = c.Candidates[:1]
	c.Criteria[0] = c.Criteria[0][:1]
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	got := record.WriterInput("", "").Listings[0].ReasonText
	if got != "Cumple los filtros publicados y el aviso describe la propiedad como luminosa." {
		t.Fatalf("confirmed requirement omitted: %s", got)
	}
	if !record.Project().Branches[0].Shown[0].Fit.NoPreferenceAdvantage {
		t.Fatal("no-preference accounting changed")
	}
}

func TestQuietContradictionWriterAddsNoUnpublishedScopeAndKeepsConditionalPunctuation(t *testing.T) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c comparison.Case
	for _, v := range cases {
		if v.ID == "lone-result" {
			c = v
		}
	}
	c.Candidates[0].Listing.Description = "Departamento sobre avenida."
	c.Candidates[0].Rules[0].Hardness = eligibility.Discretionary
	c.Candidates[0].Rules[0].Evidence = "Garantía sujeta a aprobación del propietario."
	record, err := comparison.RunProposed(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	text := record.Fallback("", "")
	if strings.Contains(text, "dentro del departamento") {
		t.Fatalf("unpublished noise scope: %s", text)
	}
	if !strings.Contains(text, "La información del aviso contradice tu preferencia de silencio.") {
		t.Fatalf("negative preference omitted: %s", text)
	}
	if strings.Contains(text, "..") || !strings.Contains(text, "Depende de aprobación: Garantía sujeta a aprobación del propietario.") {
		t.Fatalf("conditional punctuation: %s", text)
	}
}
