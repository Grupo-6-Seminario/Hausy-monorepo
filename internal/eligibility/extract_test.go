package eligibility_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
)

// fakeJev answers with canned choices and records what it was asked.
type fakeJev struct {
	answers map[string]jev.Answer
	asked   []string
	calls   int
}

func (f *fakeJev) evaluate(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
	f.calls++
	out := map[string]jev.Answer{}
	for id := range qs {
		f.asked = append(f.asked, id)
		out[id] = f.answers[id]
	}
	slices.Sort(f.asked)
	return out, nil
}

func choice(c string) jev.Answer { return jev.Answer{Type: "choice", Choice: c} }

// From a snapshot listing (Congreso).
const listingA = "Contrato residencial por 2 años\nAjuste trimestral por IPC\n\nRequisitos:\nIngresos comprobables\nGarantía propietaria\nExpensas comunes, ABL, AYSA y servicios a cargo del inquilino."

func TestExtractAsksOnlyAboutSpottedInstruments(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"instrument_propietaria": choice("accepted"), "hardness": choice("hard")}}
	rules, err := eligibility.Extract(context.Background(), f.evaluate, listingA)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.asked, []string{"hardness", "instrument_propietaria"}) {
		t.Fatalf("asked %v", f.asked)
	}
	if len(rules) != 1 || rules[0].Fact != "guarantee" || rules[0].Operator != "one_of" || !slices.Equal(rules[0].Values, []string{"propietaria"}) || rules[0].Hardness != "hard" || rules[0].Evidence != "Garantía propietaria" {
		t.Fatalf("got %+v", rules)
	}
}

// From a snapshot listing (Monserrat).
const listingB = "No se permite perro\nRequisitos y condiciones: contrato a 2 años con incremento trimestral IPC CABA. 1 mes deposito en garantía en Dólares + 1 mes adelanto + 1 garantía Caba o seguro de caución (ver cuales permite la propietaria) + es obligatorio el seguro de hogar completo."

func TestExtractKeepsEveryAcceptedInstrumentAndTheOwnersDiscretion(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"instrument_propietaria": choice("accepted"), "instrument_caucion": choice("accepted"), "hardness": choice("discretionary")}}
	rules, err := eligibility.Extract(context.Background(), f.evaluate, listingB)
	if err != nil || len(rules) != 1 || !slices.Equal(rules[0].Values, []string{"propietaria", "caucion"}) || rules[0].Hardness != "discretionary" {
		t.Fatalf("got %+v, %v", rules, err)
	}
}

func TestExtractMakesNoCallWhenNothingIsMentioned(t *testing.T) {
	f := &fakeJev{}
	rules, err := eligibility.Extract(context.Background(), f.evaluate, "Hermoso 2 ambientes luminoso, balcón al frente.")
	if err != nil || rules != nil || f.calls != 0 {
		t.Fatalf("want no rules and no call, got %+v, %d calls, %v", rules, f.calls, err)
	}
}

func TestExtractReadsTheIncomeMultipleAndLetsJevConfirmIt(t *testing.T) {
	text := "Requisitos:\nGarantía propietaria\nIngresos que tripliquen el valor del alquiler"
	f := &fakeJev{answers: map[string]jev.Answer{"instrument_propietaria": choice("accepted"), "hardness": choice("hard"), "income_multiple": {Type: "boolean", Probability: 0.97}}}
	rules, err := eligibility.Extract(context.Background(), f.evaluate, text)
	if err != nil || len(rules) != 2 {
		t.Fatalf("want guarantee and income rules, got %+v, %v", rules, err)
	}
	income := rules[1]
	if income.Fact != "income_band" || income.Operator != "income_multiple" || !slices.Equal(income.Values, []string{"3"}) || income.Evidence != "Ingresos que tripliquen el valor del alquiler" {
		t.Fatalf("got %+v", income)
	}
	f.answers["income_multiple"] = jev.Answer{Type: "boolean", Probability: 0.1}
	if rules, _ := eligibility.Extract(context.Background(), f.evaluate, text); len(rules) != 1 {
		t.Fatalf("an unconfirmed multiple must not become a rule, got %+v", rules)
	}
}

// Real snapshot lines the first extraction run missed (experiments/eligibility, L00 L01 L02 L05).
func TestExtractSpotsPropietariaWrittenOtherWays(t *testing.T) {
	for _, line := range []string{
		"-Garantía preferentemente propietaria en CABA también con recibos de ingresos verificables.",
		"GARANTIA DE CAPITAL",
		"Listo para ocupar. Garante Fiador propietario y/o fiadores a satisfacción del locador. Alquiler $550.000",
		"GARANTIA: GARANTE CON PROPIEDAD EN CABA",
	} {
		f := &fakeJev{answers: map[string]jev.Answer{"instrument_propietaria": choice("accepted"), "hardness": choice("hard")}}
		if _, err := eligibility.Extract(context.Background(), f.evaluate, line); err != nil || !slices.Contains(f.asked, "instrument_propietaria") {
			t.Errorf("not spotted: %q (asked %v)", line, f.asked)
		}
	}
}
