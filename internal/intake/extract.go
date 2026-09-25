package intake

import (
	"regexp"
	"strconv"
	"strings"
)

// Deterministic extraction: literal names and numbers are found in code; Jev
// only judges the role each one plays. Ported from experiments/intake.

// ponytail: CABA's 48 barrios plus Congreso, in code; move to data when the
// search leaves CABA.
var barrios = []string{"agronomia", "almagro", "balvanera", "barracas", "belgrano", "boedo", "caballito", "chacarita", "coghlan", "colegiales", "congreso", "constitucion", "flores", "floresta", "la boca", "la paternal", "liniers", "mataderos", "monte castro", "monserrat", "nueva pompeya", "nunez", "palermo", "parque avellaneda", "parque chacabuco", "parque chas", "parque patricios", "puerto madero", "recoleta", "retiro", "saavedra", "san cristobal", "san nicolas", "san telmo", "velez sarsfield", "versalles", "villa crespo", "villa del parque", "villa devoto", "villa general mitre", "villa lugano", "villa luro", "villa ortuzar", "villa pueyrredon", "villa real", "villa riachuelo", "villa santa rita", "villa soldati", "villa urquiza"}

var fold = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ñ", "n")

func normalize(s string) string { return fold.Replace(strings.ToLower(s)) }

type mention struct {
	Turn  int
	Text  string
	Value float64
	USD   bool
}

var (
	numberRe = regexp.MustCompile(`(?:(usd|u\$s|us\$)\s*)?\b(\d{1,3}(?:\.\d{3})+|\d+(?:,\d+)?)\s*(mil|k|lucas|millones|millon|palos?|m2|mts2?|metros|ambientes?|amb|dormitorios?|dorm|banos?|usd|dolares|pesos)?\b`)
	wordRe   = regexp.MustCompile(`\b(un|uno|una|dos|tres|cuatro|cinco)\s+(palo|millon|ambientes?|dormitorios?|banos?)\b|\bmonoambiente\b`)
	words    = map[string]float64{"un": 1, "uno": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5}
)

func multiplier(unit string) float64 {
	switch {
	case unit == "mil" || unit == "k" || unit == "lucas":
		return 1e3
	case unit == "millones" || unit == "millon" || strings.HasPrefix(unit, "palo"):
		return 1e6
	}
	return 1
}

// extractNumbers finds quantity mentions. A bare number directly followed by
// "y"/"a" and a number with a unit inherits that unit ("entre 700 y 900 mil").
func extractNumbers(turns []string) []mention {
	var out []mention
	for t, turn := range turns {
		s := normalize(turn)
		usdMessage := strings.Contains(s, "dolar") || strings.Contains(s, "usd") || strings.Contains(s, "u$s")
		locs := numberRe.FindAllStringSubmatchIndex(s, -1)
		for i, l := range locs {
			digits := s[l[4]:l[5]]
			unit := ""
			if l[6] >= 0 {
				unit = s[l[6]:l[7]]
			}
			if unit == "" && i+1 < len(locs) && locs[i+1][6] >= 0 {
				if between := strings.TrimSpace(s[l[1]:locs[i+1][0]]); between == "y" || between == "a" || between == "-" {
					unit = s[locs[i+1][6]:locs[i+1][7]]
				}
			}
			v, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(digits, ".", ""), ",", "."), 64)
			if err != nil {
				continue
			}
			prefixUSD := l[2] >= 0
			usd := prefixUSD || unit == "usd" || unit == "dolares" || (usdMessage && multiplier(unit) > 1) || (usdMessage && unit == "")
			out = append(out, mention{Turn: t + 1, Text: strings.TrimSpace(s[l[0]:l[1]]), Value: v * multiplier(unit), USD: usd})
		}
		for _, l := range wordRe.FindAllStringSubmatchIndex(s, -1) {
			if l[2] < 0 { // monoambiente
				out = append(out, mention{Turn: t + 1, Text: "monoambiente", Value: 1})
				continue
			}
			unit := s[l[4]:l[5]]
			out = append(out, mention{Turn: t + 1, Text: s[l[0]:l[1]], Value: words[s[l[2]:l[3]]] * multiplier(unit), USD: usdMessage && multiplier(unit) > 1})
		}
	}
	return out
}

func mentionedBarrios(turns []string) []string {
	text := " " + normalize(strings.Join(turns, " \n ")) + " "
	var out []string
	for _, b := range barrios {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(b) + `\b`).MatchString(text) {
			out = append(out, b)
		}
	}
	return out
}

var spotted = []struct{ re, typ, value string }{
	{`pileta|piscina`, "amenity", "pileta"}, {`gimnasio|\bgym\b`, "amenity", "gimnasio"},
	{`laundry|lavadero`, "amenity", "laundry"}, {`coworking`, "amenity", "coworking"},
	{`\bsum\b|salon de usos`, "amenity", "sum"}, {`seguridad|vigilancia|porteria`, "amenity", "seguridad"},
	{`parrilla`, "amenity", "parrilla"}, {`ascensor`, "amenity", "ascensor"},
	{`cochera|garage|estacionamiento`, "amenity", "cochera"}, {`solarium`, "amenity", "solarium"},
	{`terraza (comun|compartida)`, "amenity", "terraza_comun"},
	{`(subte|linea) a\b`, "transit_access", "subte_a"}, {`(subte|linea) b\b`, "transit_access", "subte_b"},
	{`(subte|linea) c\b`, "transit_access", "subte_c"}, {`(subte|linea) d\b`, "transit_access", "subte_d"},
	{`(subte|linea) e\b`, "transit_access", "subte_e"}, {`(subte|linea) h\b`, "transit_access", "subte_h"},
	{`\btren\b`, "transit_access", "tren"}, {`colectivo|bondi`, "transit_access", "colectivo"},
}

// habitacionRe finds a room count said in habitaciones, which are dormitorios
// (CONTEXT.md), over normalized text.
var habitacionRe = regexp.MustCompile(`\b(\d+|un|una|dos|tres|cuatro|cinco)\s+habitacion(?:es)?\b`)

// habitaciones returns the last count the conversation gave in habitaciones.
func habitaciones(conversation string) (int, bool) {
	all := habitacionRe.FindAllStringSubmatch(conversation, -1)
	if len(all) == 0 {
		return 0, false
	}
	said := all[len(all)-1][1]
	if n, err := strconv.Atoi(said); err == nil {
		return n, n > 0
	}
	return int(words[said]), true
}

func spottedNames(turns []string) []int {
	text := normalize(strings.Join(turns, " \n "))
	var out []int
	for i, s := range spotted {
		if regexp.MustCompile(s.re).MatchString(text) {
			out = append(out, i)
		}
	}
	return out
}
