package buyer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type pendingClarification struct {
	turns     []string
	plan      intake.Plan
	questions []clarification.Question
	message   string
	confirmed []clarification.Effect
	resolved  []string
}

const clarificationPreviewLimit = 50

type previewCandidates interface {
	PreviewCandidates(context.Context, search.Query, int) ([]eligibility.Candidate, error)
}

type attributeCoverage interface {
	HasAttributeData(context.Context, search.Query, string) (bool, error)
}

type boundedInventory struct{ Inventory }

func (b boundedInventory) Candidates(ctx context.Context, q search.Query) ([]eligibility.Candidate, error) {
	if preview, ok := b.Inventory.(previewCandidates); ok {
		return preview.PreviewCandidates(ctx, q, clarificationPreviewLimit)
	}
	all, err := b.Inventory.Candidates(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(all) > clarificationPreviewLimit {
		all = all[:clarificationPreviewLimit]
	}
	return all, nil
}

type ClarificationAnswer struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected,omitempty"`
	Other      string   `json:"other,omitempty"`
	Action     string   `json:"action,omitempty"` // edit | remove | decline
}

func retryQuestion(q clarification.Question) *TurnResponse {
	return &TurnResponse{Clarification: &q, ClarificationHint: "No pude interpretar esa respuesta. Elegí una opción o escribí un dato más preciso."}
}

func logClarification(ctx context.Context, outcome string, started time.Time) {
	logging.FromContext(ctx).LogAttrs(ctx, slog.LevelInfo, "buyer_clarification",
		slog.String("outcome", outcome), slog.Int64("duration_ms", time.Since(started).Milliseconds()))
}

func (a *DefaultAgent) PendingClarification(sessionID string) *clarification.Question {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sess := a.sessions[sessionID]; sess != nil && sess.pending != nil && len(sess.pending.questions) > 0 {
		q := sess.pending.questions[0]
		return &q
	}
	return nil
}

func (a *DefaultAgent) HandleClarification(ctx context.Context, sessionID string, answer ClarificationAnswer, declared eligibility.Qualification, events Events) (*TurnResponse, error) {
	started := time.Now()
	a.mu.Lock()
	sess := a.sessions[sessionID]
	if sess == nil || sess.pending == nil || len(sess.pending.questions) == 0 || sess.pending.questions[0].ID != answer.QuestionID {
		a.mu.Unlock()
		return nil, ErrStaleClarification
	}
	pending := sess.pending
	q := pending.questions[0]
	if answer.Action == "edit" {
		sess.pending = nil
		a.mu.Unlock()
		logClarification(ctx, "edited", started)
		return &TurnResponse{}, nil
	}
	var effects []clarification.Effect
	switch {
	case q.Kind == "unsupported" && q.CanRemove && answer.Action == "remove":
		pending.plan = withoutInvalidAttributes(pending.plan)
		if q.Source != q.Request {
			// A later plan must not re-read the removed phrase. A generic stop
			// cites the whole request, which must not read as removing it all.
			pending.turns = append(pending.turns, "Quito la condición «"+q.Source+"» de mi búsqueda.")
		}
	case q.Kind == "qualification" && answer.Action == "decline":
		if sess.declinedFacts == nil {
			sess.declinedFacts = map[string]bool{}
		}
		sess.declinedFacts[q.Fact] = true
		delete(pending.plan.Qualification, q.Fact)
	case answer.Action != "":
		a.mu.Unlock()
		return nil, ErrStaleClarification
	case answer.Other != "":
		normalizer, ok := a.pipeline.clarifier.(clarification.Normalizer)
		if !ok {
			a.mu.Unlock()
			return retryQuestion(q), nil
		}
		a.mu.Unlock()
		var err error
		effects, err = normalizer.Normalize(ctx, q, answer.Other)
		if err != nil {
			return retryQuestion(q), nil
		}
		a.mu.Lock()
		if a.sessions[sessionID] != sess || sess.pending != pending || pending.questions[0].ID != answer.QuestionID {
			a.mu.Unlock()
			return nil, ErrStaleClarification
		}
		allowed := map[string][]string{}
		for _, c := range q.Choices {
			for _, e := range c.Effects {
				allowed[effectTarget(e)] = slices.Clone(e.Clear)
			}
		}
		for i, e := range effects {
			clear, ok := allowed[effectTarget(e)]
			if !ok {
				a.mu.Unlock()
				return retryQuestion(q), nil
			}
			effects[i].Clear = clear
		}
	default:
		if q.Kind == "unsupported" || len(answer.Selected) == 0 || !q.Multi && len(answer.Selected) != 1 {
			a.mu.Unlock()
			return nil, ErrStaleClarification
		}
		seen := map[string]bool{}
		for _, id := range answer.Selected {
			if seen[id] {
				a.mu.Unlock()
				return nil, ErrStaleClarification
			}
			seen[id] = true
			found := false
			for _, c := range q.Choices {
				if c.ID == id {
					effects = append(effects, c.Effects...)
					found = true
					break
				}
			}
			if !found {
				a.mu.Unlock()
				return nil, ErrStaleClarification
			}
		}
	}
	if len(effects) > 0 {
		plan, err := clarification.Apply(pending.plan, effects)
		if err != nil {
			a.mu.Unlock()
			return retryQuestion(q), nil
		}
		pending.plan = plan
		for _, effect := range effects {
			effect.Clear = nil
			pending.confirmed = append(pending.confirmed, effect)
		}
	}
	pending.resolved = append(pending.resolved, strings.ToLower(q.Source))
	pending.questions = pending.questions[1:]
	for len(pending.questions) > 0 {
		next := pending.questions[0]
		if next.Kind != "unsupported" {
			a.mu.Unlock()
			impact, unsupported, checkErr := a.questionImpact(ctx, pending.plan, declared, next)
			a.mu.Lock()
			if sess.pending != pending {
				a.mu.Unlock()
				return nil, ErrStaleClarification
			}
			if checkErr == nil && unsupported {
				next.Kind, next.Choices, next.CanRemove = "unsupported", nil, canRemove(pending.plan)
				next.Prompt = "Todavía no puedo aplicar «" + next.Source + "» a los avisos. Editá la búsqueda para continuar."
				if next.CanRemove {
					next.Prompt = "Todavía no puedo aplicar «" + next.Source + "» a los avisos. Podés editar la búsqueda o quitar esa condición."
				}
				pending.questions[0] = next
			}
			if checkErr == nil && !impact && !unsupported {
				pending.questions = pending.questions[1:]
				continue
			}
		}
		a.mu.Unlock()
		return &TurnResponse{Clarification: &next}, nil
	}
	for _, branch := range pending.plan.Branches {
		if _, err := branch.Validate(); err != nil {
			followup := unsupportedQuestion(pending.message, "", pending.plan)
			pending.questions = []clarification.Question{followup}
			a.mu.Unlock()
			return &TurnResponse{Clarification: &followup}, nil
		}
	}
	if sess.resolvedSources == nil {
		sess.resolvedSources = map[string]bool{}
	}
	for _, source := range pending.resolved {
		sess.resolvedSources[source] = true
	}
	sess.confirmed = append(sess.confirmed, pending.confirmed...)
	sess.pending = nil
	sess.inFlight = true
	// The reply answers every turn since the last search, removals included,
	// so it never reports a removed condition as unmet.
	message := strings.Join(pending.turns[len(sess.turns):], "\n")
	a.mu.Unlock()
	logClarification(ctx, "completed", started)
	return a.handlePipeline(ctx, sessionID, message, declared, events, pending)
}

func effectTarget(e clarification.Effect) string {
	if e.Branch == nil {
		return e.Field
	}
	return e.Field + "#" + strconv.Itoa(*e.Branch)
}

func (a *DefaultAgent) prepareClarification(ctx context.Context, sess *session, turns []string, plan intake.Plan, declared eligibility.Qualification, planErr error) (*clarification.Question, bool, error) {
	started := time.Now()
	problem := ""
	if planErr != nil {
		problem = planErr.Error()
	}
	proposals, err := a.pipeline.clarifier.Propose(ctx, turns, plan, problem, mergeQualification(declared, plan.Qualification))
	if err != nil {
		proposals = nil
	}
	var candidates []clarification.Question
	asksAmenities := false
	for _, q := range proposals {
		if clarification.Validate(q, turns) != nil || q.Kind == "qualification" && (sess.declinedFacts[q.Fact] || len(declared[q.Fact]) > 0) {
			continue
		}
		for _, c := range q.Choices {
			for _, e := range c.Effects {
				asksAmenities = asksAmenities || strings.HasPrefix(e.Value, "amenity=")
			}
		}
		candidates = append(candidates, q)
	}
	if q, ok := clarification.UnnamedAmenities(planErr); ok && !asksAmenities {
		candidates = append(candidates, q)
	}
	var questions []clarification.Question
	for _, q := range candidates {
		if sess.resolvedSources[strings.ToLower(q.Source)] && !strings.Contains(strings.ToLower(turns[len(turns)-1]), strings.ToLower(q.Source)) {
			continue
		}
		if q.Kind != "unsupported" {
			impact, unsupported, checkErr := a.questionImpact(ctx, plan, declared, q)
			if checkErr != nil {
				return nil, false, checkErr
			}
			if unsupported {
				q.Kind = "unsupported"
				q.Choices = nil
				q.Prompt = "Todavía no puedo aplicar «" + q.Source + "» a los avisos. Podés editar la búsqueda o quitar esa condición."
			}
			if !impact && !unsupported {
				continue
			}
		}
		q.ID = newQuestionID()
		q.Request = turns[len(turns)-1]
		if q.Kind == "unsupported" {
			q.CanRemove = canRemove(plan)
			if !q.CanRemove {
				q.Prompt = "Todavía no puedo aplicar «" + q.Source + "» a los avisos. Editá la búsqueda para continuar."
			}
		}
		questions = append(questions, q)
	}
	if phrase := knownDroppedRequirement(turns[len(turns)-1], plan); len(questions) == 0 && (planErr != nil || phrase != "") {
		questions = append(questions, unsupportedQuestion(turns[len(turns)-1], phrase, plan))
	}
	if len(questions) == 0 {
		logClarification(ctx, "not_needed", started)
		return nil, false, nil
	}
	// A pending question is the only writer of this session until it is
	// answered or cancelled. The ID also makes old responses harmless.
	a.mu.Lock()
	sess.pending = &pendingClarification{turns: slices.Clone(turns), plan: plan, questions: questions, message: turns[len(turns)-1]}
	a.mu.Unlock()
	logClarification(ctx, "asked", started)
	return &questions[0], true, nil
}

// unsupportedQuestion stops the search. It cites phrase when a known one
// caused the stop, and the whole request otherwise.
func unsupportedQuestion(request, phrase string, plan intake.Plan) clarification.Question {
	q := clarification.Question{ID: newQuestionID(), Request: request, Kind: "unsupported", Source: request, CanRemove: canRemove(plan)}
	if phrase != "" {
		q.Source = phrase
		q.Prompt = "Todavía no puedo aplicar «" + phrase + "» a los avisos. Editá la búsqueda para continuar."
		if q.CanRemove {
			q.Prompt = "Todavía no puedo aplicar «" + phrase + "» a los avisos. Podés editar la búsqueda o quitar esa condición."
		}
		return q
	}
	q.Prompt = "No puedo aplicar con seguridad una condición de tu búsqueda. Editala para continuar."
	if q.CanRemove {
		q.Prompt = "No puedo aplicar con seguridad una condición de tu búsqueda. Editala o quitá esa condición para continuar."
	}
	return q
}

func canRemove(plan intake.Plan) bool {
	if len(plan.Branches) == 0 {
		return false
	}
	for _, branch := range withoutInvalidAttributes(plan).Branches {
		if _, err := branch.Validate(); err != nil {
			return false
		}
	}
	return true
}

func newQuestionID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes[:])
}

// knownDroppedRequirement returns the phrase of a known hard condition the
// plan cannot carry, or "" when there is none.
func knownDroppedRequirement(latest string, plan intake.Plan) string {
	text := strings.ToLower(latest)
	for _, phrase := range []string{"cerca del trabajo", "cerca de mi trabajo"} {
		if strings.Contains(text, phrase) {
			return phrase
		}
	}
	if strings.Contains(text, "con amenities") {
		for _, b := range plan.Branches {
			for _, f := range b.RequiredAttributes {
				if f.Type == "amenity" {
					return ""
				}
			}
		}
		return "con amenities"
	}
	return ""
}

func preserveConfirmed(plan intake.Plan, effects []clarification.Effect, latest string) (intake.Plan, []clarification.Effect) {
	var keep []clarification.Effect
	for _, effect := range effects {
		if explicitlyChanges(latest, effect) {
			continue
		}
		updated, err := clarification.Apply(plan, []clarification.Effect{effect})
		if err != nil {
			continue
		}
		plan = updated
		keep = append(keep, effect)
	}
	return plan, keep
}

func explicitlyChanges(message string, effect clarification.Effect) bool {
	text := strings.ToLower(message)
	switch effect.Field {
	case "rooms_exact", "min_bedrooms":
		return strings.Contains(text, "ambiente") || strings.Contains(text, "dormitorio") || strings.Contains(text, "habitaci")
	case "required_attribute", "preferred_attribute", "excluded_attribute":
		typ, value, _ := strings.Cut(effect.Value, "=")
		return strings.Contains(text, strings.ReplaceAll(value, "_", " ")) || strings.Contains(text, typ) && (strings.Contains(text, "sin ") || strings.Contains(text, "olvidate") || strings.Contains(text, "ya no"))
	case "currency", "max_price":
		return strings.ContainsAny(text, "0123456789$") || strings.Contains(text, "peso") || strings.Contains(text, "dólar") || strings.Contains(text, "dolar") || strings.Contains(text, "usd")
	case "qualification.guarantee":
		return strings.Contains(text, "garantía") || strings.Contains(text, "garantia") || strings.Contains(text, "caución") || strings.Contains(text, "caucion")
	case "qualification.income_band":
		return strings.Contains(text, "ingreso") || strings.Contains(text, "sueldo") || strings.Contains(text, "gano ")
	case "qualification.caucion_quoted":
		return strings.Contains(text, "cotiz")
	}
	return false
}

func withoutInvalidAttributes(plan intake.Plan) intake.Plan {
	plan.Branches = slices.Clone(plan.Branches)
	for i := range plan.Branches {
		b := &plan.Branches[i]
		b.RequiredAttributes = slices.DeleteFunc(slices.Clone(b.RequiredAttributes), func(f search.AttributeFilter) bool { return !listing.AllowsValue(f.Type, f.Value) })
		b.PreferredAttributes = slices.DeleteFunc(slices.Clone(b.PreferredAttributes), func(f search.AttributeFilter) bool { return !listing.AllowsValue(f.Type, f.Value) })
		b.ExcludedAttributes = slices.DeleteFunc(slices.Clone(b.ExcludedAttributes), func(f search.AttributeFilter) bool { return !listing.AllowsValue(f.Type, f.Value) })
	}
	return plan
}

func (a *DefaultAgent) questionImpact(ctx context.Context, plan intake.Plan, declared eligibility.Qualification, q clarification.Question) (bool, bool, error) {
	base := withoutInvalidAttributes(plan)
	if len(base.Branches) == 0 {
		return false, true, nil
	}
	preview := *a.pipeline
	preview.inventory = boundedInventory{a.pipeline.inventory}
	preview.matcher = nil
	// A parsed attribute vocabulary alone does not imply that the current
	// inventory carries this fact. No searchable data means no honest question.
	checked := map[string]bool{}
	for _, c := range q.Choices {
		for _, e := range c.Effects {
			if e.Field != "required_attribute" && e.Field != "preferred_attribute" && e.Field != "excluded_attribute" {
				continue
			}
			typ, _, _ := strings.Cut(e.Value, "=")
			if checked[typ] {
				continue
			}
			checked[typ] = true
			// Coverage is inventory-wide: unsupported means no listing
			// anywhere can confirm this attribute (CONTEXT.md). A branch
			// whose listings lack it leaves them unconfirmed, and the
			// impact check below decides whether asking is worth it.
			everywhere := search.Query{Operation: base.Branches[0].Operation}
			found := false
			if coverage, ok := a.pipeline.inventory.(attributeCoverage); ok {
				var err error
				if found, err = coverage.HasAttributeData(ctx, everywhere, typ); err != nil {
					return false, false, err
				}
			} else {
				candidates, err := a.pipeline.inventory.Candidates(ctx, everywhere)
				if err != nil {
					return false, false, err
				}
				for _, candidate := range candidates {
					found = found || slices.ContainsFunc(candidate.Listing.Attributes, func(attr listing.Attribute) bool { return attr.Type == typ })
				}
			}
			if !found {
				return false, true, nil
			}
		}
	}
	catalog, err := a.pipeline.inventory.Facts(ctx)
	if err != nil {
		return false, false, err
	}
	baseline, _, _, _, err := preview.rank(ctx, base, mergeQualification(declared, base.Qualification), catalog)
	if err != nil {
		return false, false, err
	}
	signatures := map[string]bool{}
	if q.Kind == "qualification" {
		signatures[resultSignature(baseline)] = true
	} // decline preserves unknown eligibility
	var allEffects []clarification.Effect
	for _, choice := range q.Choices {
		allEffects = append(allEffects, choice.Effects...)
		variant, err := clarification.Apply(base, choice.Effects)
		if err != nil {
			continue
		}
		valid := true
		for _, branch := range variant.Branches {
			if _, err := branch.Validate(); err != nil {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		shown, _, _, _, err := preview.rank(ctx, variant, mergeQualification(declared, variant.Qualification), catalog)
		if err != nil {
			return false, false, err
		}
		signatures[resultSignature(shown)] = true
		if len(signatures) > 1 {
			return true, false, nil
		}
	}
	if q.Multi && len(allEffects) > 1 {
		variant, err := clarification.Apply(base, allEffects)
		if err == nil {
			shown, _, _, _, err := preview.rank(ctx, variant, mergeQualification(declared, variant.Qualification), catalog)
			if err != nil {
				return false, false, err
			}
			signatures[resultSignature(shown)] = true
		}
	}
	return len(signatures) > 1, false, nil
}

func resultSignature(results []Result) string {
	var b strings.Builder
	for _, result := range results {
		b.WriteString(result.URL)
		if result.Eligibility != nil {
			b.WriteString(string(result.Eligibility.State))
		}
		b.WriteByte('\n')
	}
	return b.String()
}
