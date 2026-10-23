package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func TestCandidatesReturnEveryHardFilterMatchWithItsRules(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	save := func(url, hood string, price float64) {
		t.Helper()
		if err := store.Save(ctx, listing.Listing{Source: "zonaprop", URL: url, Neighborhood: hood, Description: "Departamento.", Operation: "alquiler",
			Price: listing.Money{Amount: float64Ptr(price), Currency: "ARS"}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveReview(ctx, url, quality.Review{Status: quality.Passed}); err != nil {
			t.Fatal(err)
		}
	}
	save("https://ex.test/a", "palermo", 800000)
	save("https://ex.test/b", "palermo", 1200000)
	save("https://ex.test/c", "caballito", 700000)
	rule := eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria", "caucion"}, Hardness: "discretionary", Visibility: "public", Source: "parsed", Evidence: "Garantía propietaria o caución (ver cuáles)"}
	if err := store.SaveEligibility(ctx, "https://ex.test/a", []eligibility.Rule{rule}); err != nil {
		t.Fatal(err)
	}
	// Saving again replaces rather than duplicates.
	if err := store.SaveEligibility(ctx, "https://ex.test/a", []eligibility.Rule{rule}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}, Currency: "ARS", MaxPrice: float64Ptr(1000000)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Listing.URL != "https://ex.test/a" || len(got[0].Rules) != 1 || !slices.Equal(got[0].Rules[0].Values, rule.Values) || got[0].Rules[0].Evidence != rule.Evidence {
		t.Fatalf("got %+v", got)
	}
	all, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil || len(all) != 2 {
		t.Fatalf("want both Palermo listings uncapped, got %d, %v", len(all), err)
	}
	preview, err := store.PreviewCandidates(ctx, search.Query{Neighborhoods: []string{"palermo"}}, 1)
	if err != nil || len(preview) != 1 || preview[0].Listing.URL != "https://ex.test/a" {
		t.Fatalf("the impact preview must obey its sample limit: %+v, %v", preview, err)
	}
	hasAmenity, err := store.HasAttributeData(ctx, search.Query{Neighborhoods: []string{"palermo"}}, "amenity")
	if err != nil || hasAmenity {
		t.Fatalf("inventory without amenity attributes cannot answer amenity questions: %v, %v", hasAmenity, err)
	}
	item := listing.Listing{Source: "zonaprop", URL: "https://ex.test/a", Neighborhood: "palermo", Description: "Departamento.", Operation: "alquiler",
		Price: listing.Money{Amount: float64Ptr(800000), Currency: "ARS"}, Attributes: []listing.Attribute{{Type: "amenity", Value: "pileta", Provenance: listing.Stated}}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	hasAmenity, err = store.HasAttributeData(ctx, search.Query{Neighborhoods: []string{"palermo"}}, "amenity")
	if err != nil || !hasAmenity {
		t.Fatalf("an approved searchable amenity should be detected: %v, %v", hasAmenity, err)
	}
}

func TestAdmissibleFactsAreDataAndExcludeProtectedCharacteristics(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !facts.Admissible("guarantee") || !facts.Admissible("income_band") || facts.Admissible("age") || facts.Admissible("nationality") {
		t.Fatalf("got %v", facts)
	}
}

// The form reads the catalog from the table (specs/004, phase 1 contract), so
// the table holds every fact's label, priority, cardinality, order and choices.
func TestFactsCarryTheFormCatalog(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	yesNo := []eligibility.Choice{{Value: "yes", Label: "Sí"}, {Value: "no", Label: "No"}}
	want := map[string]eligibility.Fact{
		"guarantee": {Admissible: true, Label: "Garantía", Priority: "high", Multiple: true, Position: 1, Choices: []eligibility.Choice{
			{Value: "propietaria", Label: "Garantía propietaria"},
			{Value: "caucion", Label: "Seguro de caución"},
			{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"},
			{Value: "none", Label: "No tengo"},
		}},
		"income_documented": {Admissible: true, Label: "¿Podés comprobar tus ingresos?", Priority: "high", Position: 2, Choices: yesNo},
		"pets": {Admissible: true, Label: "Mascotas", Priority: "high", Multiple: true, Position: 3, Choices: []eligibility.Choice{
			{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"}, {Value: "cat", Label: "Gato"},
			{Value: "other", Label: "Otra"},
		}},
		"income_band": {Admissible: true, Label: "Ingresos mensuales", Priority: "medium", Position: 4, Choices: []eligibility.Choice{
			{Value: "0-1000000", Label: "Hasta $1.000.000"},
			{Value: "1000000-2000000", Label: "$1.000.000 a $2.000.000"},
			{Value: "2000000-3000000", Label: "$2.000.000 a $3.000.000"},
			{Value: "3000000-", Label: "Más de $3.000.000"},
		}},
	}
	for name, fact := range want {
		if got := facts[name]; !reflect.DeepEqual(got, fact) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, fact)
		}
	}
	for _, protected := range []string{"age", "nationality", "gender"} {
		if facts.Admissible(protected) {
			t.Errorf("%s must stay inadmissible", protected)
		}
	}
	// Spec 007: no rule reads a caución quote, so the form stops asking it.
	if _, ok := facts["caucion_quoted"]; ok {
		t.Error("caucion_quoted must leave the catalog")
	}
}

// Spec 006, error 5: a rule with no values would hide the listing from every
// searcher, so saving it fails and the error names the listing.
func TestARuleWithNoValuesIsRefusedNamingTheListing(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	url := "https://ex.test/empty-rule"
	if err := store.Save(ctx, listing.Listing{Source: "zonaprop", URL: url, Neighborhood: "palermo", Description: "Departamento.", Operation: "alquiler",
		Price: listing.Money{Amount: float64Ptr(800000), Currency: "ARS"}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	empty := eligibility.Rule{Fact: "pets", Operator: "subset_of", Hardness: "hard"}
	err := store.SaveEligibility(ctx, url, []eligibility.Rule{empty})
	if err == nil || !strings.Contains(err.Error(), url) || !strings.Contains(err.Error(), "admits no values") {
		t.Fatalf("want the save refused naming %s, got %v", url, err)
	}
}

// A rule nobody can evaluate is corrupt data. Saving it is refused, so a bad
// eligibility file cannot break later searches; a row that still arrives by
// other means fails loudly on read instead of reaching the evaluator.
func TestARuleWithAnUnknownOperatorIsRefusedOnWriteAndOnRead(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	url := "https://ex.test/between"
	if err := store.Save(ctx, listing.Listing{Source: "zonaprop", URL: url, Neighborhood: "palermo", Description: "Departamento.", Operation: "alquiler",
		Price: listing.Money{Amount: float64Ptr(800000), Currency: "ARS"}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, url, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	valid := eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"caucion"}, Hardness: "hard"}
	if err := store.SaveEligibility(ctx, url, []eligibility.Rule{valid}); err != nil {
		t.Fatal(err)
	}
	between := eligibility.Rule{Fact: "guarantee", Operator: "between", Values: []string{"propietaria"}, Hardness: "hard"}

	if err := store.SaveEligibility(ctx, url, []eligibility.Rule{valid, between}); err == nil || !strings.Contains(err.Error(), `"between"`) {
		t.Fatalf("want the save refused naming the unknown operator, got %v", err)
	}
	got, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil || len(got) != 1 || len(got[0].Rules) != 1 || got[0].Rules[0].Operator != "one_of" {
		t.Fatalf("a refused save must leave the previous rules: %+v, %v", got, err)
	}

	conn, err := pgx.Connect(ctx, os.Getenv("HAUSY_TEST_DATABASE_URI"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `INSERT INTO listing_eligibility_rules (listing_id, fact, operator, "values", hardness)
		SELECT id, 'guarantee', 'between', '["propietaria"]', 'hard' FROM listings WHERE url = $1`, url); err != nil {
		t.Fatal(err)
	}
	got, err = store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err == nil || !strings.Contains(err.Error(), `"between"`) {
		t.Fatalf("want an error naming the unknown operator, got %+v, %v", got, err)
	}
}

func TestQualificationRoundTripsPerUser(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	user, err := store.CreateUser(ctx, auth.NewUser{Email: fmt.Sprintf("q%d@ex.test", time.Now().UnixNano()), Name: "Q", Role: auth.RoleSearcher, PasswordHash: "x"})
	if err != nil {
		t.Fatal(err)
	}
	q := eligibility.Qualification{"guarantee": {"caucion", "propietaria"}, "income_band": {"2000000-3000000"}}
	if err := store.SaveQualification(ctx, user.ID, q); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveQualification(ctx, user.ID, eligibility.Qualification{"guarantee": {"caucion"}}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Qualification(ctx, user.ID)
	if err != nil || len(got) != 1 || !slices.Equal(got["guarantee"], []string{"caucion"}) {
		t.Fatalf("saving replaces the whole qualification, got %v, %v", got, err)
	}
	if err := store.SaveQualification(ctx, user.ID, eligibility.Qualification{"age": {"19"}}); err == nil {
		t.Fatal("an inadmissible fact must be refused")
	}
}

// Candidates is read in one query instead of one per listing. Each candidate
// must be the listing ByURL reads, and its rules the ones Candidates returned
// before the change: the digests were taken from the per-listing
// implementation over the committed inventory.
func TestCandidatesKeepTheListingsAndRulesOfThePerListingReads(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	loadCommitted(t, store)
	maxPrice := 900000.0
	for _, tc := range []struct {
		name   string
		read   func() ([]eligibility.Candidate, error)
		count  int
		digest string
	}{
		// Re-captured for spec 006: listing 59530580 gains its "sin mascota" rule (error 1)
		// and 59966540's income rule covers expensas (error 2). Re-captured for spec 007:
		// the income_documented rules are the only change (without them the old digests hold).
		// Re-captured for spec 008: 59787969's caución rejection is the only change.
		{"whole inventory", func() ([]eligibility.Candidate, error) { return store.Candidates(ctx, search.Query{}) }, 279, "9a07191784c46af67a98004ac6e85974ae529687c33931b5e15b168017693467"},
		{"palermo and congreso up to 900k", func() ([]eligibility.Candidate, error) {
			return store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo", "congreso"}, Operation: "alquiler", Currency: "ARS", MaxPrice: &maxPrice})
		}, 92, "ee6482b0fba7b507f23f87e342040f6fc9b7d563d4011ed16281d26342991f15"},
		{"capped preview", func() ([]eligibility.Candidate, error) { return store.PreviewCandidates(ctx, search.Query{}, 50) }, 50, "ae66d400e4629be7e909e1db35152a4f049498121fcc785f3d56a723b43068d7"},
	} {
		got, err := tc.read()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rules := sha256.New()
		for _, c := range got {
			want, err := store.ByURL(ctx, c.Listing.URL)
			if err != nil || !reflect.DeepEqual(c.Listing, want) {
				t.Fatalf("%s: %s differs from ByURL (%v):\n got %+v\nwant %+v", tc.name, c.Listing.URL, err, c.Listing, want)
			}
			line, _ := json.Marshal(struct {
				URL   string
				Rules []eligibility.Rule
			}{c.Listing.URL, c.Rules})
			rules.Write(append(line, '\n'))
		}
		digest := hex.EncodeToString(rules.Sum(nil))
		if len(got) != tc.count || digest != tc.digest {
			t.Errorf("%s: got %d candidates, rules digest %s; want %d, %s", tc.name, len(got), digest, tc.count, tc.digest)
		}
	}
}

// Spec 006, error 3: every choice the form offers can be declared in the chat
// too, so a value added to the catalog without a way to say it fails here
// instead of being silently dropped from every chat.
func TestEveryCatalogChoiceCanBeDeclaredInTheChat(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for name, fact := range facts {
		if !fact.Admissible {
			continue
		}
		for _, c := range fact.Choices {
			declarable := slices.ContainsFunc(eligibility.Declarables, func(d eligibility.Declarable) bool {
				return d.Fact == name && (d.Value == c.Value || d.Value == "")
			})
			if !declarable {
				t.Errorf("%s=%s has no Declarable", name, c.Value)
			}
		}
	}
}

// Spec 007: after load, exactly the searchable listings whose description
// asks the tenant to prove their income carry the income_documented rule,
// each with its own clause. The list is the hand review in
// specs/007-income-proof-rule/clauses.md; a guarantor's pay slips and an
// "or" alternative (59980152, 55770968, 55360032) are not on it.
func TestLoadAsksForProofOfIncomeWhereTheListingDoes(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	loadCommitted(t, store)
	want := strings.Fields(`53113773 54669946 58495144 58918850 59104263 59127966 59159610 59242857 59273620 59277251
		59336411 59344807 59356911 59379959 59388489 59443185 59456752 59512038 59519746 59569984 59570949 59602413
		59732810 59733661 59769260 59772156 59775328 59794846 59799057 59819140 59846555 59857030 59902632 59915695
		59916237 59922514 59922516 59925532 59933400 59959677 59966540 59973061 59975659 59984791 59992529 60022997
		60028740 60029155 60030285 60030932 60031095 60032274 60033803 60045025 60045336 60047227 60054215`)
	candidates, err := store.Candidates(ctx, search.Query{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range candidates {
		for _, r := range c.Rules {
			if r.Fact != "income_documented" {
				continue
			}
			if r.Operator != eligibility.OneOf || !slices.Equal(r.Values, []string{"yes"}) || r.Hardness != eligibility.Hard || r.Evidence == "" || !strings.Contains(c.Listing.Description, r.Evidence) {
				t.Errorf("%s: want a hard one_of [yes] rule quoting the listing, got %+v", c.Listing.URL, r)
			}
			got = append(got, regexp.MustCompile(`(\d+)\.html$`).FindStringSubmatch(c.Listing.URL)[1])
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("listings with the rule:\n got %v\nwant %v", got, want)
	}
}

// Spec 008: after load, the one listing that names a guarantee it refuses
// carries the rejection, with its clause; no other listing does
// (specs/008-guarantee-rejections/clauses.md).
func TestLoadReadsTheGuaranteesAListingRefuses(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	loadCommitted(t, store)
	candidates, err := store.Candidates(ctx, search.Query{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range candidates {
		for _, r := range c.Rules {
			if r.Operator != eligibility.NoneOf {
				continue
			}
			got = append(got, fmt.Sprintf("%s %s %v %s %q", regexp.MustCompile(`(\d+)\.html$`).FindStringSubmatch(c.Listing.URL)[1], r.Fact, r.Values, r.Hardness, r.Evidence))
		}
	}
	want := []string{`59787969 guarantee [caucion] hard "No se aceptan seguros de caución"`}
	if !slices.Equal(got, want) {
		t.Errorf("rejections:\n got %v\nwant %v", got, want)
	}
}
