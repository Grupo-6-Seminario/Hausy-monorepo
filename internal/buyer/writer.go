package buyer

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// LocalWriter explains the ranking with one local-model call: no tools, and
// only the packet as material.
type LocalWriter struct{ Client llm.Client }

const writerPrompt = `Sos el agente de Hausy. Te paso, en JSON, una búsqueda que YA está resuelta y ordenada.
Tu única tarea es explicar por qué las propiedades mostradas son una buena opción para esta persona.

Reglas:
- Las propiedades ya vienen ordenadas por elegibilidad (podés aplicar → depende de algo por confirmar → sin datos de requisitos). Nunca las reordenes ni inventes otras.
- Referenciá cada propiedad con su rank ("#1") tal cual viene, para que coincida con las tarjetas.
- Para cada una: su elegibilidad, citando textual la evidencia del aviso: la de "met" (lo que la persona ya cumple) y la de "conditions" (lo que falta confirmar). Después, qué requisitos de la persona cumple, citando la evidencia de los atributos. "stated" es palabra del aviso; "inferred" es una lectura: no la afirmes como hecho.
- En "branches", "matches" cuenta los avisos que cumplen todo lo pedido, también los que no se muestran por sus requisitos; "unconfirmed" cuenta los que se muestran igual pero no confirman una comodidad pedida. Nunca los sumes como si cumplieran.
- "qualitative_fit=exact" significa que hay apoyo textual para la cualidad pedida; "unconfirmed" significa que es sólo una alternativa y debés decir que esa cualidad falta confirmar. Nunca presentes un indicio (frente, orientación, ventanas) como prueba.
- Quién decide una condición: "discretionary" la decide la inmobiliaria. "near_line" es un requisito fijo de ingresos y la banda de ingresos de la persona queda a ambos lados del umbral: depende de su ingreso exacto, así que decile que lo compare con la cuenta del aviso; nunca digas que la inmobiliaria decide ni que tiene margen.
- "declared" es lo que la persona dijo que tiene. Si su garantía es "none", decí que declaró no tener garantía (nunca que no sabemos o no tenemos información sobre su garantía).
- "unknown" es que no sabemos si puede aplicar: o el aviso no publica requisitos, o publica uno que la persona no nos dijo si cumple (una condición "missing" o "unverifiable": citala y decí qué dato falta). Nunca digas que puede aplicar. Una condición "unconfirmed" es un aviso que rechaza otras garantías sin decir que acepta la de la persona: citala y decí que el aviso no dice qué garantías acepta; nunca digas que rechaza la de la persona.
- Si hay relaxations, contá cuántas propiedades vuelven con esa garantía ("si conseguís <garantía>, vuelven N"). Sin relaxations, nunca digas que otra garantía devolvería propiedades.
- Si hay "hidden_note", decilo una sola vez, tal cual: son las mismas propiedades que "matches" ya cuenta, nunca las sumes ni las presentes como otras.
- De las propiedades que no se muestran no conocés sus requisitos: nunca supongas qué piden o aceptan, ni por qué quedaron afuera, ni qué otra garantía o dato las devolvería ("es posible que", "quizás pida"). Sólo podés decir "hidden_note" y, si hay, las relaxations.
- Si una rama tiene 0 resultados, decilo con el barrio. Podés sugerir aflojar alguno de sus requirements, nunca otro barrio.
- Todo lo que digas sale del JSON: no nombres ni sugieras barrios que no estén ahí (tampoco "barrios cercanos"), no afirmes precios, stock ni datos del mercado, y no generalices sobre el barrio o el mercado. Nunca expliques estas reglas ni digas qué no podés hacer.
- Escribí en castellano llano: no uses las claves ni los valores del JSON (unknown, eligible, stated, inferred, hard, discretionary, near_line, unconfirmed, declared, none, relaxations, qualitative_fit).
- Si intent es "ask_about_listing", respondé la pregunta usando sólo los datos de esas propiedades.
- Formato: siempre "## Mi lectura" (una o dos frases); si hay propiedades, "## Por qué las elegí" (hasta tres); "## Qué falta confirmar" sólo si hace falta (hasta dos puntos). Sin tablas. Español rioplatense.`

func (w LocalWriter) Write(ctx context.Context, p Packet, reply func(delta string)) (string, error) {
	// The row id is for the card's contact intent; the model cites ranks.
	p.Shown = slices.Clone(p.Shown)
	for i := range p.Shown {
		p.Shown[i].ID = 0
	}
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

// templateReply is the deterministic brief used when the writer fails, so a
// model outage never costs the user their results.
// verdictLabel words a verdict as the result card does. A conditional one
// names who decides: the searcher's exact income for a band that straddles a
// hard threshold, the agency for a discretionary rule. An unknown one says
// why: the ad publishes nothing, or asks for something the searcher has not
// declared or that cannot be checked.
func verdictLabel(v eligibility.Verdict) string {
	if v.State == eligibility.ConditionallyEligible {
		reasons := map[eligibility.Reason]bool{}
		for _, c := range v.Conditions {
			reasons[c.Reason] = true
		}
		switch {
		case reasons[eligibility.NearLine] && reasons[eligibility.DiscretionaryCall]:
			return "depende de tus ingresos y de la inmobiliaria"
		case reasons[eligibility.NearLine]:
			return "depende de tus ingresos"
		}
	}
	if v.State != eligibility.Unknown || len(v.Conditions) == 0 {
		return stateLabel[v.State]
	}
	for _, c := range v.Conditions {
		if c.Reason == "missing" {
			return "pide un requisito que no nos dijiste si cumplís"
		}
	}
	// A rejection that spares the searcher's guarantee, or their lack of one,
	// never said which guarantees it accepts.
	if slices.ContainsFunc(v.Conditions, func(c eligibility.Condition) bool { return c.Reason == eligibility.Unconfirmed }) {
		return "no dice qué garantías acepta"
	}
	return "pide un requisito que no se puede verificar"
}

// hiddenNote words the hidden count for the model, singular or plural.
func hiddenNote(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "1 propiedad no se muestra porque no cumplís sus requisitos."
	}
	return fmt.Sprintf("%d propiedades no se muestran porque no cumplís sus requisitos.", n)
}

func templateReply(p Packet) string {
	var b strings.Builder
	b.WriteString("## Mi lectura\n")
	if len(p.Shown) == 0 {
		b.WriteString("No encontré propiedades que cumplan todo lo que pediste.\n")
	} else {
		if p.Shown[0].QualitativeFit == "unconfirmed" {
			b.WriteString("No encontré coincidencias exactas con evidencia para todas las cualidades pedidas. Estas son opciones que faltan confirmar.\n")
		} else {
			b.WriteString("Ordené las propiedades según si podés alquilarlas.\n")
		}
	}
	if len(p.Shown) > 0 {
		b.WriteString("\n## Por qué las elegí\n")
	}
	for _, r := range p.Shown {
		title := r.Address
		if title == "" {
			title = "Departamento en " + r.Neighborhood
		}
		line := fmt.Sprintf("- **#%d %s**", r.Rank, title)
		if r.Eligibility != nil {
			line += ": " + verdictLabel(*r.Eligibility)
			for _, c := range r.Eligibility.Conditions {
				if c.Rule.Evidence != "" {
					line += fmt.Sprintf(" (\"%s\")", c.Rule.Evidence)
				}
			}
		}
		b.WriteString(line + ".\n")
	}
	var notes []string
	for _, r := range p.Relaxations {
		label := strings.ToLower(r.Label)
		if label == "" {
			label = r.Value
		}
		back := fmt.Sprintf("vuelven %d propiedades", r.Count)
		if r.Count == 1 {
			back = "vuelve 1 propiedad"
		}
		notes = append(notes, fmt.Sprintf("Si conseguís %s, %s.", label, back))
	}
	for _, br := range p.Branches {
		if br.Matches == 0 {
			notes = append(notes, fmt.Sprintf("%s: 0 avisos con lo que pediste.", strings.Join(br.Neighborhoods, ", ")))
		}
	}
	if len(notes) > 0 {
		b.WriteString("\n## Qué falta confirmar\n- " + strings.Join(notes, "\n- ") + "\n")
	}
	return b.String()
}
