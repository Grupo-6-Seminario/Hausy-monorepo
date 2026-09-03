package buyer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
)

// Option configures an Agent at construction.
type Option func(*DefaultAgent)

// WithInventory gives the agent a listing store to search.
//
// Without it the agent only extracts requirements from what the user says --
// useful before there is anything to search. With it, the model is handed the
// read tools and answers from the inventory instead of from its own priors.
func WithInventory(repository search.Repository) Option {
	return func(agent *DefaultAgent) {
		registry := tools.NewRegistry()
		registry.MustRegister(search.Toolset(repository)...)
		agent.inventory = repository
		agent.runner = tools.NewRunner(agent.llmClient, registry, tools.DefaultMaxSteps)
	}
}

// inventorySystemPrompt is the whole of the agent's judgement, so it is written
// as instructions rather than as a description: what to decide before calling a
// tool, what never to claim, and what to do when nothing matches.
//
// It is in Spanish because the user's half of the conversation is.
const inventorySystemPrompt = `Sos un agente inmobiliario que ayuda a buscar propiedades en Buenos Aires.

Trabajás sobre un inventario real. Consultalo con las herramientas; nunca inventes propiedades,
precios ni barrios.

Cómo buscar:
- Usá search_listings para todo. Las restricciones duras del usuario (precio, barrio, ambientes)
  van como filtros; lo que preferiría pero negociaría va en preferred_attributes, que ordena los
  resultados sin descartar ninguno.
- Si no sabés qué barrios existen, llamá a list_neighborhoods antes de buscar. Un barrio inventado
  no da error: da cero resultados.
- Toda cota de precio necesita currency. ARS y USD conviven sin tipo de cambio.
- Si total_matches es mucho mayor que las filas que recibiste, acotá antes de responder.
- Si no hay resultados, no te quedes ahí: contá qué filtro los dejó afuera y ofrecé aflojarlo.
- Usá get_listing sobre las dos o tres que estés comparando en serio, y neighborhood_price_stats
  cuando quieras decir si un precio es caro o barato para la zona.

Cómo responder:
- Comparás en base a lo que devuelven las herramientas. Los atributos "stated" son palabras del
  aviso; los "inferred" son una lectura de esas palabras: citá la evidencia en vez de afirmarlos.
- Un campo vacío significa que el aviso no lo publicó, no que valga cero. Decilo así.
- Respondé en español rioplatense, en pocas frases, explicando por qué esas propiedades y no otras.
- Preguntá lo que te falte, de a una cosa por vez.`

// handleWithInventory runs one turn against the inventory: the model searches,
// reads and compares through the toolset, and the listings it settled on come
// back alongside its reply.
func (a *DefaultAgent) handleWithInventory(ctx context.Context, sessionID, message string) (*TurnResponse, error) {
	a.mu.Lock()
	sess, ok := a.sessions[sessionID]
	if !ok {
		sess = &session{id: sessionID}
		a.sessions[sessionID] = sess
	}
	history := append([]llm.Message(nil), sess.history...)
	a.mu.Unlock()

	if len(history) == 0 {
		history = append(history, llm.Message{Role: "system", Content: inventorySystemPrompt})
	}
	history = append(history, llm.Message{Role: "user", Content: message})

	result, err := a.runner.Run(ctx, llm.ChatRequest{
		Messages:    history,
		Temperature: 0,
		MaxTokens:   1024,
	})
	if err != nil {
		return nil, fmt.Errorf("buyer: inventory turn failed: %w", err)
	}

	query, matches, searched := lastSearch(result.Steps)
	listings := a.expand(ctx, matches)

	requirements := []Requirement{}
	if searched {
		requirements = requirementsFromQuery(query)
	}

	a.mu.Lock()
	sess.history = result.Messages
	sess.requirements = requirements
	a.mu.Unlock()

	return &TurnResponse{
		Reply:        result.Reply,
		Requirements: requirements,
		Listings:     listings,
	}, nil
}

// lastSearch finds the search the model settled on. A turn often widens or
// narrows more than once, and what belongs on screen is where it stopped, not
// everything it tried on the way.
func lastSearch(steps []tools.Step) (search.Query, []search.Match, bool) {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Call.Name != "search_listings" {
			continue
		}

		var query search.Query
		if err := json.Unmarshal(steps[i].Call.Arguments, &query); err != nil {
			continue
		}
		var results search.Results
		if err := json.Unmarshal([]byte(steps[i].Result), &results); err != nil {
			// A rejected call carries {"error": ...} and no matches; keep
			// looking back for one that actually ran.
			continue
		}
		if validated, err := query.Validate(); err == nil {
			query = validated
		}
		return query, results.Matches, true
	}
	return search.Query{}, nil, false
}

// expand turns the search rows into whole listings for the interface. A search
// row is trimmed for the model's context; the cards the user reads want the
// full description and the evidence behind each attribute.
func (a *DefaultAgent) expand(ctx context.Context, matches []search.Match) []listing.Listing {
	listings := make([]listing.Listing, 0, len(matches))
	for _, match := range matches {
		item, err := a.inventory.ByURL(ctx, match.URL)
		if err != nil {
			// One unreadable row should not cost the user the other nine.
			continue
		}
		listings = append(listings, item)
	}
	return listings
}

// requirementsFromQuery renders the search that ran as the requirements the
// interface shows.
//
// Reading them off the real query rather than asking the model to restate them
// costs no extra call and cannot drift: what the user sees listed is what was
// actually asked of the database.
func requirementsFromQuery(query search.Query) []Requirement {
	requirements := make([]Requirement, 0, 8)
	add := func(kind, value string) {
		if value != "" {
			requirements = append(requirements, Requirement{Type: kind, Value: value})
		}
	}

	for _, neighborhood := range query.Neighborhoods {
		add("neighborhood", neighborhood)
	}
	add("operation", query.Operation)
	if query.MinPrice != nil {
		add("min_price", formatAmount(*query.MinPrice)+" "+query.Currency)
	}
	if query.MaxPrice != nil {
		add("max_price", formatAmount(*query.MaxPrice)+" "+query.Currency)
	}
	if query.MaxExpensesARS != nil {
		add("max_expenses", formatAmount(*query.MaxExpensesARS)+" ARS")
	}
	for kind, value := range map[string]*int{
		"min_rooms": query.MinRooms, "max_rooms": query.MaxRooms,
		"min_bedrooms": query.MinBedrooms, "min_bathrooms": query.MinBathrooms,
		"min_parking_spaces": query.MinParkingSpaces, "max_age_years": query.MaxAgeYears,
	} {
		if value != nil {
			add(kind, strconv.Itoa(*value))
		}
	}
	if query.MinTotalAreaM2 != nil {
		add("min_total_area_m2", formatAmount(*query.MinTotalAreaM2))
	}
	for _, filter := range query.RequiredAttributes {
		add(filter.Type, filter.Value)
	}
	for _, filter := range query.PreferredAttributes {
		add("prefiere_"+filter.Type, filter.Value)
	}
	for _, filter := range query.ExcludedAttributes {
		add("excluye_"+filter.Type, filter.Value)
	}

	// Map iteration above is unordered; a list that reshuffles between turns
	// looks to the user like the agent changed its mind.
	sortRequirements(requirements)
	return requirements
}

func formatAmount(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
}

func sortRequirements(requirements []Requirement) {
	sort.SliceStable(requirements, func(i, j int) bool {
		if requirements[i].Type != requirements[j].Type {
			return requirements[i].Type < requirements[j].Type
		}
		return requirements[i].Value < requirements[j].Value
	})
}
