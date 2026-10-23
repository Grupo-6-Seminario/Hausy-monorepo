package eligibility

import (
	"regexp"
	"strings"
)

// Declarable is how a searcher says, in the chat, that they have a catalog
// value. Value "" stands for every value of Fact: the model reads which one,
// and Says only confirms the searcher spoke about the fact. The catalog owns
// the facts and their values; this table owns the words that declare them.
type Declarable struct {
	Fact, Value string
	// Description tells a model what the value is, in English.
	Description string
	Says        func(text string) bool
}

func matches(pattern string) func(string) bool {
	re := regexp.MustCompile(pattern)
	return re.MatchString
}

// denies opens a statement that the searcher lacks something.
const denies = `\b(?:no\s+(?:tengo|cuento\s+con|poseo)|sin)\b`

var (
	incomeAmountCue = regexp.MustCompile(`(?i)\d|mill[oó]n|palo|luca`)
)

// declaresIncomeAmount is an income cue followed closely by an amount, in the
// same clause: "gano 2 millones", "mis ingresos son de 1.500.000".
func declaresIncomeAmount(text string) bool {
	for _, clause := range strings.FieldsFunc(text, func(r rune) bool { return r == '.' || r == ';' || r == '?' || r == '!' || r == '\n' }) {
		for _, at := range topics["income_band"].FindAllStringIndex(clause, -1) {
			end := min(len(clause), at[1]+48)
			if incomeAmountCue.MatchString(clause[at[1]:end]) {
				return true
			}
		}
	}
	return false
}

// Declarables lists every way the chat can declare a qualification value.
// Guarantees reuse Instruments' patterns; a value without an entry here can
// only be declared in the form.
var Declarables = declarables()

func declarables() []Declarable {
	var out []Declarable
	for _, in := range Instruments {
		out = append(out, Declarable{Fact: "guarantee", Value: in.Name, Description: in.Description, Says: in.Pattern.MatchString})
	}
	return append(out,
		Declarable{Fact: "guarantee", Value: "none", Description: "no rental guarantee of any kind",
			Says: matches(`(?i)` + denies + `[^.;]{0,30}?\bgarant[ií]as?\b`)},
		Declarable{Fact: "pets", Value: "dog", Description: "a dog", Says: matches(`(?i)\bperr[oa]s?\b`)},
		Declarable{Fact: "pets", Value: "cat", Description: "a cat", Says: matches(`(?i)\bgat[oa]s?\b`)},
		Declarable{Fact: "pets", Value: "other", Description: "a pet that is neither a dog nor a cat", Says: matches(`(?i)\bmascotas?\b|\banimal(?:es)?\b`)},
		Declarable{Fact: "pets", Value: "none", Description: "no pets", Says: matches(`(?i)` + denies + `[^.;]{0,30}?\b(?:mascotas?|perr[oa]s?|gat[oa]s?|animal(?:es)?)\b`)},
		Declarable{Fact: "income_band", Description: "their monthly income", Says: declaresIncomeAmount},
		Declarable{Fact: "income_documented", Description: "whether they can document their income", Says: matches(`(?i)comprob|demostr|en\s+blanco|en\s+negro|informal`)},
	)
}

// declarable is the entry that lets the chat declare value of fact.
func declarable(fact, value string) (Declarable, bool) {
	for _, d := range Declarables {
		if d.Fact == fact && (d.Value == value || d.Value == "") {
			return d, true
		}
	}
	return Declarable{}, false
}

// Declared keeps what a model read from the chat only where the catalog offers
// it and the searcher's own words say it: a model was seen inventing an income
// band, and a declared fact changes eligibility. A single-choice fact keeps its
// first value; an offered value without a Declarable is dropped, never guessed.
func Declared(c Catalog, q Qualification, text string) Qualification {
	out := Qualification{}
	for fact, values := range q {
		for _, v := range values {
			if !c.Admissible(fact) || !c.Offers(fact, v) {
				continue
			}
			if d, ok := declarable(fact, v); ok && d.Says(text) {
				out[fact] = append(out[fact], v)
			}
		}
		if len(out[fact]) > 1 && !c[fact].Multiple {
			out[fact] = out[fact][:1]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// topics are the words a searcher uses when they talk about a fact at all,
// whatever value they mean: "tengo garantía" names the guarantee without
// saying which.
var topics = map[string]*regexp.Regexp{
	"guarantee":         regexp.MustCompile(`(?i)garant|cauci|fianza|garante`),
	"income_band":       regexp.MustCompile(`(?i)ingreso|sueldo|\bgan(o|amos)\b|\bcobr|facturo`),
	"pets":              regexp.MustCompile(`(?i)mascota|perr[oa]|gat[oa]|animal`),
	"income_documented": regexp.MustCompile(`(?i)comprob|demostr|recibo|en\s+blanco|en\s+negro|informal`),
}

// Mentions reports whether text talks about fact, so a confirmed answer the
// searcher brings up again is read afresh. A fact without a topic is never
// mentioned.
func Mentions(fact, text string) bool {
	topic, ok := topics[fact]
	return ok && topic.MatchString(text)
}
