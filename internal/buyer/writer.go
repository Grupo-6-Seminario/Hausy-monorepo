package buyer

import (
	"context"
	"encoding/json"
	"fmt"
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
		label := instrumentLabel[r.Value]
		if label == "" {
			label = r.Value
		}
		notes = append(notes, fmt.Sprintf("Si conseguís %s, vuelven %d propiedades.", label, r.Count))
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
