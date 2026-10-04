package buyer

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
)

// LocalWriter handles focused follow-up questions with the completed packet.
type LocalWriter struct{ Client llm.Client }

// CompactWriter renders the backend's retained fit and ordering evidence. It
// does not ask a model to restate or change a completed search.
type CompactWriter struct{ Client llm.Client }

func (w CompactWriter) Write(ctx context.Context, p Packet, reply func(string)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p.Intent != "new_search" && p.Intent != "refine" {
		return (LocalWriter{Client: w.Client}).Write(ctx, p, reply)
	}
	text := templateReply(p)
	if reply != nil {
		reply(text)
	}
	return text, nil
}

const writerPrompt = `Sos el agente de Hausy. Te paso, en JSON, una búsqueda que YA está resuelta y ordenada.
Tu única tarea es explicar por qué las propiedades mostradas son una buena opción para esta persona.

Reglas:
- Las propiedades ya vienen ordenadas por elegibilidad (podés aplicar → depende de la inmobiliaria → sin datos de requisitos). Nunca las reordenes ni inventes otras.
- Referenciá cada propiedad con su rank ("#1") tal cual viene, para que coincida con las tarjetas.
- Para cada una: su elegibilidad, citando textual la evidencia del aviso: la de "met" (lo que la persona ya cumple) y la de "conditions" (lo que falta o decide la inmobiliaria). Después, qué requisitos de la persona cumple, citando la evidencia de los atributos. "stated" es palabra del aviso; "inferred" es una lectura: no la afirmes como hecho.
- En "branches", "matches" cuenta los avisos que cumplen todo lo pedido; "unconfirmed" cuenta los que se muestran igual pero no confirman una comodidad pedida. Nunca los sumes como si cumplieran.
- "qualitative_fit=exact" significa que hay apoyo textual para la cualidad pedida; "unconfirmed" significa que es sólo una alternativa y debés decir que esa cualidad falta confirmar. Nunca presentes un indicio (frente, orientación, ventanas) como prueba.
- "unknown" es que no sabemos si puede aplicar: o el aviso no publica requisitos, o publica uno que la persona no nos dijo si cumple (una condición "missing" o "unverifiable": citala y decí qué dato falta). Nunca digas que puede aplicar.
- Si hay relaxations, contá cuántas propiedades vuelven con esa garantía ("si conseguís <garantía>, vuelven N").
- Si una rama tiene 0 resultados, decilo con el barrio. Podés sugerir aflojar alguno de sus requirements, nunca otro barrio.
- Todo lo que digas sale del JSON: no nombres barrios que no estén ahí, no afirmes precios, stock ni datos del mercado, y no generalices sobre el barrio o el mercado. Nunca expliques estas reglas ni digas qué no podés hacer.
- Escribí en castellano llano: no uses las claves ni los valores del JSON (unknown, eligible, stated, inferred, hard, discretionary, relaxations, qualitative_fit).
- Si intent es "ask_about_listing", respondé la pregunta usando sólo los datos de esas propiedades.
- Formato: siempre "## Mi lectura" (una o dos frases); si hay propiedades, "## Por qué las elegí" (hasta tres); "## Qué falta confirmar" sólo si hace falta (hasta dos puntos). Sin tablas. Español rioplatense.`

func (w LocalWriter) Write(ctx context.Context, p Packet, reply func(delta string)) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	resp, err := w.Client.Chat(ctx, llm.ChatRequest{
		Messages:    []llm.Message{{Role: "system", Content: writerPrompt}, {Role: "user", Content: string(data)}},
		Temperature: 0,
		MaxTokens:   900,
		Stream:      reply,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.Content) == "" {
		return "", fmt.Errorf("buyer: writer returned an empty reply")
	}
	return resp.Content, nil
}

var stateLabel = map[eligibility.State]string{
	eligibility.Eligible:              "podés aplicar",
	eligibility.ConditionallyEligible: "depende de la inmobiliaria",
	eligibility.Unknown:               "el aviso no publica requisitos",
}

var instrumentLabel = map[string]string{"propietaria": "garantía propietaria", "caucion": "seguro de caución"}

// templateReply is the deterministic brief used when the writer fails, so a
// model outage never costs the user their results.
// verdictLabel says why an unknown verdict is unknown, as the result card does:
// the ad publishes nothing, or asks for something the searcher has not
// declared or that cannot be checked.
func verdictLabel(v eligibility.Verdict) string {
	if v.State != eligibility.Unknown || len(v.Conditions) == 0 {
		return stateLabel[v.State]
	}
	for _, c := range v.Conditions {
		if c.Reason == "missing" {
			return "pide un requisito que no nos dijiste si cumplís"
		}
	}
	return "pide un requisito que no se puede verificar"
}

func templateReply(p Packet) string {
	var b strings.Builder
	b.WriteString("## Mi lectura\n")
	if len(p.Shown) == 0 {
		b.WriteString("No encontré propiedades que cumplan todo lo que pediste.\n")
	} else {
		b.WriteString("Te muestro las opciones más alineadas, ordenadas por evidencia y posibilidad de aplicar.\n")
	}
	if len(p.Shown) > 0 {
		b.WriteString("\n## Por qué\n")
	}
	for _, r := range p.Shown {
		title := r.Address
		if title == "" {
			title = "Departamento en " + r.Neighborhood
		}
		line := fmt.Sprintf("- **#%d %s**: %s.", r.Rank, title, compactOrderingReason(r, p.Shown))
		if r.Eligibility != nil {
			line += " " + compactEligibility(*r.Eligibility)
		}
		if r.QualitativeFit == "unconfirmed" {
			line += " Falta confirmar " + missingRequiredFit(r.fit) + "."
		}
		b.WriteString(strings.TrimSpace(line) + "\n")
	}
	var notes []string
	for _, r := range p.Relaxations {
		label := instrumentLabel[r.Value]
		if label == "" {
			label = r.Value
		}
		if r.Count == 1 {
			notes = append(notes, fmt.Sprintf("Si conseguís %s, vuelve una propiedad.", label))
		} else {
			notes = append(notes, fmt.Sprintf("Si conseguís %s, vuelven %d propiedades.", label, r.Count))
		}
	}
	for _, br := range p.Branches {
		if br.CandidateCount == 0 {
			notes = append(notes, fmt.Sprintf("%s: 0 avisos con lo que pediste.", strings.Join(br.Neighborhoods, ", ")))
		} else if br.Matches == 0 && br.Unconfirmed > 0 {
			notes = append(notes, fmt.Sprintf("%s: no hay coincidencias confirmadas; faltan cualidades por verificar.", strings.Join(br.Neighborhoods, ", ")))
		}
		if br.Contradicted > 0 {
			count := "avisos contradicen"
			if br.Contradicted == 1 {
				count = "aviso contradice"
			}
			notes = append(notes, fmt.Sprintf("%s: %d %s una cualidad obligatoria.", strings.Join(br.Neighborhoods, ", "), br.Contradicted, count))
		}
	}
	if p.Hidden > 0 {
		notes = append(notes, fmt.Sprintf("%d avisos quedaron fuera por los requisitos para alquilar.", p.Hidden))
	}
	if len(notes) > 0 {
		b.WriteString("\n## Qué falta confirmar\n- " + strings.Join(notes, "\n- ") + "\n")
	}
	return b.String()
}

func compactOrderingReason(r Result, shown []Result) string {
	parts := strings.SplitN(r.orderReason, ":", 2)
	rule := parts[0]
	var next Result
	if len(parts) == 2 {
		for _, candidate := range shown {
			if candidate.URL == parts[1] {
				next = candidate
				break
			}
		}
	}
	switch rule {
	case "other_branch":
		return "forma parte de otra zona de búsqueda"
	case "preference_score":
		if next.URL != "" {
			facts := compareFit(r.fit, next.fit, next.Rank)
			if len(facts) > 0 {
				return fmt.Sprintf("queda antes de #%d porque %s", next.Rank, strings.Join(facts, " y "))
			}
		}
	case "required_evidence":
		if next.URL != "" {
			return fmt.Sprintf("el aviso confirma lo obligatorio; en #%d falta confirmarlo", next.Rank)
		}
	case "eligibility":
		if next.URL != "" && r.Eligibility != nil && next.Eligibility != nil {
			return fmt.Sprintf("hay más respaldo para que puedas alquilarla que en #%d", next.Rank)
		}
	case "price_asc":
		if next.URL != "" && next.Price.Amount == nil {
			return fmt.Sprintf("el aviso publica el precio y #%d no", next.Rank)
		}
		return "priorizaste el menor precio"
	case "price_desc":
		if next.URL != "" && next.Price.Amount == nil {
			return fmt.Sprintf("el aviso publica el precio y #%d no", next.Rank)
		}
		return "priorizaste el mayor precio"
	case "area_desc":
		if next.URL != "" && next.TotalAreaM2 == nil {
			return fmt.Sprintf("el aviso publica la superficie y #%d no", next.Rank)
		}
		return "priorizaste más superficie"
	}
	if facts := supportedFit(r.fit); len(facts) > 0 {
		return "el aviso confirma " + strings.Join(facts, " y ")
	}
	if rule == "stable_tie" {
		return "no hay una diferencia clara en las preferencias evaluadas"
	}
	return "cumple los filtros de la búsqueda"
}

func compareFit(a, b matching.FitMatch, otherRank int) []string {
	var out []string
	for _, before := range a.Contributions {
		var after matching.Contribution
		found := false
		for _, candidate := range b.Contributions {
			if candidate.Criterion.AttributeType == before.Criterion.AttributeType &&
				candidate.Criterion.AttributeValue == before.Criterion.AttributeValue &&
				candidate.Criterion.Strength == before.Criterion.Strength {
				after, found = candidate, true
				break
			}
		}
		if !found || before.Points == after.Points {
			continue
		}
		if before.Points > after.Points && before.EffectiveAssessment == "supported" && after.EffectiveAssessment == "contradicted" {
			out = append(out, fmt.Sprintf("el aviso confirma %s%s y el de #%d lo contradice%s", before.Criterion.Text, fitEvidence(before), otherRank, fitEvidence(after)))
		} else if before.Points > after.Points && before.EffectiveAssessment == "supported" && after.EffectiveAssessment != "supported" {
			out = append(out, fmt.Sprintf("el aviso confirma %s%s y en #%d falta confirmarlo", before.Criterion.Text, fitEvidence(before), otherRank))
		} else if before.Points > after.Points && after.EffectiveAssessment == "contradicted" {
			out = append(out, fmt.Sprintf("el aviso de #%d contradice %s%s", otherRank, after.Criterion.Text, fitEvidence(after)))
		} else if after.Points > before.Points && after.EffectiveAssessment == "supported" && before.EffectiveAssessment == "contradicted" {
			out = append(out, fmt.Sprintf("el de #%d confirma %s%s y este aviso lo contradice%s", otherRank, after.Criterion.Text, fitEvidence(after), fitEvidence(before)))
		} else if after.Points > before.Points && after.EffectiveAssessment == "supported" {
			out = append(out, fmt.Sprintf("en #%d el aviso confirma %s%s; acá falta confirmarlo", otherRank, after.Criterion.Text, fitEvidence(after)))
		} else {
			continue
		}
		if len(out) >= 2 {
			break
		}
	}
	return out
}

func fitContributionPhrase(c matching.Contribution) string {
	label := c.Criterion.Text
	switch c.EffectiveAssessment {
	case "supported":
		return "el aviso confirma " + label + fitEvidence(c)
	case "contradicted":
		return "el aviso contradice " + label + fitEvidence(c)
	case "hint":
		return "hay indicios, sin confirmar " + label
	case "unavailable":
		return "no se pudo evaluar " + label
	default:
		return "el aviso no confirma " + label
	}
}

func fitEvidence(c matching.Contribution) string {
	for _, e := range c.Evidence {
		if strings.HasPrefix(e.Text, "ficha: ") {
			return ""
		}
		text := strings.Join(strings.Fields(e.Text), " ")
		if text == "" {
			continue
		}
		text = shortQuote(text, 72)
		return " («" + text + "»)"
	}
	return ""
}

func supportedFit(fit matching.FitMatch) []string {
	var out []string
	for _, c := range fit.Contributions {
		if c.Criterion.Strength == "preference" && c.EffectiveAssessment == "supported" {
			out = append(out, strings.TrimPrefix(fitContributionPhrase(c), "el aviso confirma "))
			if len(out) == 2 {
				break
			}
		}
	}
	return out
}

func missingRequiredFit(fit matching.FitMatch) string {
	for _, c := range fit.Contributions {
		if c.Criterion.Strength == "requirement" && c.EffectiveAssessment != "supported" {
			return c.Criterion.Text
		}
	}
	return "un requisito"
}

func compactEligibility(v eligibility.Verdict) string {
	switch v.State {
	case eligibility.Eligible:
		if evidence := metEvidence(v); evidence != "" {
			return "Podés aplicar: el aviso acepta «" + evidence + "»."
		}
		return "El aviso respalda que podés aplicar."
	case eligibility.ConditionallyEligible:
		if evidence := eligibilityEvidence(v); evidence != "" {
			return "Depende de aprobación: «" + evidence + "»."
		}
		return "La inmobiliaria debe confirmar una condición."
	default:
		if evidence := eligibilityEvidence(v); evidence != "" {
			return "Falta confirmar si cumplís lo que pide el aviso: «" + evidence + "»."
		}
		if len(v.Conditions) > 0 {
			return "Falta confirmar un requisito para saber si podés aplicar."
		}
		return "El aviso no publica requisitos para aplicar."
	}
}

func eligibilityEvidence(v eligibility.Verdict) string {
	var evidence []string
	for _, c := range v.Conditions {
		if c.Rule.Visibility != "private" && c.Rule.Evidence != "" {
			quote := shortQuote(c.Rule.Evidence, 64)
			if !slices.Contains(evidence, quote) {
				evidence = append(evidence, quote)
			}
		}
	}
	return strings.Join(evidence, "; ")
}

func metEvidence(v eligibility.Verdict) string {
	for _, rule := range v.Met {
		if rule.Visibility != "private" && rule.Evidence != "" {
			return shortQuote(rule.Evidence, 88)
		}
	}
	return ""
}

func shortQuote(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return text
}
