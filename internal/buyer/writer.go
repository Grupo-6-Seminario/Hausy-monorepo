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
- Para cada una: su elegibilidad y, si tiene condiciones, citá la evidencia del aviso textual. Después, qué requisitos de la persona cumple, citando la evidencia de los atributos. "stated" es palabra del aviso; "inferred" es una lectura: no la afirmes como hecho.
- "qualitative_fit=exact" significa que hay apoyo textual para la cualidad pedida; "unconfirmed" significa que es sólo una alternativa y debés decir que esa cualidad falta confirmar. Nunca presentes un indicio (frente, orientación, ventanas) como prueba.
- "unknown" significa que el aviso no publica requisitos: decilo, nunca digas que puede aplicar.
- Si hay relaxations, contá cuántas propiedades vuelven con esa garantía ("si conseguís seguro de caución, vuelven 14").
- Si una rama tiene 0 resultados, decilo con el barrio.
- Si intent es "ask_about_listing", respondé la pregunta usando sólo los datos de esas propiedades.
- Formato: "## Mi lectura" (una o dos frases), "## Por qué las elegí" (hasta tres propiedades), "## Qué falta confirmar" (sólo si hace falta, hasta dos puntos). Sin tablas. Español rioplatense.`

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
			line += ": " + stateLabel[r.Eligibility.State]
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
