package buyer_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

type failingWriter struct{}

func (failingWriter) Write(context.Context, buyer.Packet, func(string)) (string, error) {
	return "", errors.New("local model down")
}

func TestAFailedWriterStillExplainsTheRanking(t *testing.T) {
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), failingWriter{})
	resp := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	for _, want := range []string{"#1", "#2", "#3", "ver cuáles permite la propietaria", "caución"} {
		if !strings.Contains(resp.Reply, want) {
			t.Fatalf("template reply is missing %q:\n%s", want, resp.Reply)
		}
	}
}

type recordingLLM struct{ last llm.ChatRequest }

func (r *recordingLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	r.last = req
	return &llm.ChatResponse{Content: "## Mi lectura\n#1 es la mejor."}, nil
}

func TestLocalWriterExplainsFromThePacketInOneCall(t *testing.T) {
	client := &recordingLLM{}
	w := &fakeWriter{}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), w)
	turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})

	reply, err := buyer.LocalWriter{Client: client}.Write(context.Background(), w.packets[0], nil)
	if err != nil || reply != "## Mi lectura\n#1 es la mejor." {
		t.Fatalf("got %q, %v", reply, err)
	}
	prompt := client.last.Messages[len(client.last.Messages)-1].Content
	if !strings.Contains(prompt, `"rank":1`) || !strings.Contains(prompt, "ver cuáles permite la propietaria") {
		t.Fatalf("the writer must receive the packet: %+v", client.last)
	}
}

// Models copy the prompt's examples verbatim: the 9B once wrote "vuelven 14"
// for a relaxation whose count was 1, and Sonnet offered "seguro de caución"
// to a searcher who already had one. Only a rank may be a number there, and
// no guarantee may be named.
func TestLocalWriterExamplesCarryNoCountOrGuaranteeAModelCouldCopy(t *testing.T) {
	client := &recordingLLM{}
	if _, err := (buyer.LocalWriter{Client: client}).Write(context.Background(), buyer.Packet{Intent: "new_search"}, nil); err != nil {
		t.Fatal(err)
	}
	var system string
	for _, m := range client.last.Messages {
		if m.Role == "system" {
			system += m.Content
		}
	}
	quoted, rank, digit := regexp.MustCompile(`"[^"]*"`), regexp.MustCompile(`#\d+`), regexp.MustCompile(`\d`)
	guarantee := regexp.MustCompile(`(?i)cauci[oó]n|propietari[ao]`)
	for _, example := range quoted.FindAllString(system, -1) {
		if digit.MatchString(rank.ReplaceAllString(example, "")) {
			t.Errorf("example %s carries a number a model could copy", example)
		}
		if guarantee.MatchString(example) {
			t.Errorf("example %s names a guarantee a model could copy", example)
		}
	}
}

// streamingWriter emits deltas, then fails if err is set.
type streamingWriter struct {
	deltas []string
	err    error
}

func (w streamingWriter) Write(_ context.Context, _ buyer.Packet, reply func(string)) (string, error) {
	for _, d := range w.deltas {
		reply(d)
	}
	if w.err != nil {
		return "", w.err
	}
	return strings.Join(w.deltas, ""), nil
}

// The ranking takes a few seconds and the reply several more: the searcher
// gets the cards first and reads the reply as it is written.
func TestATurnReportsItsRankingBeforeStreamingTheReply(t *testing.T) {
	var log []string
	var early buyer.TurnResponse
	events := buyer.Events{
		Results: func(r buyer.TurnResponse) { early = r; log = append(log, "results") },
		Reply:   func(d string) { log = append(log, "reply:"+d) },
	}
	q := eligibility.Qualification{"guarantee": {"propietaria"}}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), streamingWriter{deltas: []string{"#1 ", "es la mejor"}})
	resp, err := agent.HandleMessage(context.Background(), "s", "Alquiler en Palermo", q, events)
	if err != nil || strings.Join(log, ",") != "results,reply:#1 ,reply:es la mejor" || len(early.Listings) != 3 || early.Reply != "" || early.HiddenIneligible != 1 || resp.Reply != "#1 es la mejor" {
		t.Fatalf("got events %v, early %+v, reply %q, %v", log, early, resp.Reply, err)
	}

	// A writer that dies mid-reply: the template replaces the partial text.
	cut := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), streamingWriter{deltas: []string{"#1 es"}, err: errors.New("cut")})
	resp, err = cut.HandleMessage(context.Background(), "s", "Alquiler en Palermo", q, events)
	if err != nil || !strings.Contains(resp.Reply, "## Mi lectura") || !strings.Contains(resp.Reply, "#3") {
		t.Fatalf("want the template reply, got %q, %v", resp.Reply, err)
	}
}

// Seen live: the template said "el aviso no publica requisitos" and then
// quoted the requirement the ad publishes.
func TestTemplateSaysAPublishedRequirementIsPublished(t *testing.T) {
	inventory := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("asks", "palermo", 500000, caucionOnly),
		candidate("silent", "palermo", 600000, nil),
	}}}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, failingWriter{})
	resp := turn(t, agent, "Alquiler en Palermo", nil)
	for _, line := range strings.Split(resp.Reply, "\n") {
		quotesRule := strings.Contains(line, caucionOnly[0].Evidence)
		if strings.HasPrefix(line, "- **#") && quotesRule == strings.Contains(line, "no publica requisitos") {
			t.Fatalf("only the silent listing may say it publishes no requirements:\n%s", resp.Reply)
		}
	}
}

// labelledStock is fourStates with the catalog's guarantee labels.
type labelledStock struct{ stock }

func (labelledStock) Facts(context.Context) (eligibility.Catalog, error) {
	return eligibility.Catalog{"guarantee": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
		{Value: "propietaria", Label: "Garantía propietaria"},
		{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"},
	}}}, nil
}

// Spec 006, error 4: the zero-results line names the instrument as the
// catalog does, in the template and in what the model is given.
func TestTheZeroResultsLineNamesTheInstrumentFromTheCatalog(t *testing.T) {
	slips := []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"recibos_garante"}, Hardness: "hard"}}
	inventory := labelledStock{stock{byHood: map[string][]eligibility.Candidate{"palermo": {candidate("slips", "palermo", 700000, slips)}}}}
	q := eligibility.Qualification{"guarantee": {"propietaria"}}

	template := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, failingWriter{}), "Alquiler en Palermo", q)
	if !strings.Contains(template.Reply, "Si conseguís recibos de sueldo de un garante, vuelve 1 propiedad.") {
		t.Errorf("template reply:\n%s", template.Reply)
	}

	model := &recordingLLM{}
	turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, buyer.LocalWriter{Client: model}), "Alquiler en Palermo", q)
	packet := model.last.Messages[len(model.last.Messages)-1].Content
	if !strings.Contains(packet, "Recibos de sueldo de un garante") {
		t.Errorf("the model's packet must carry the label, got:\n%s", packet)
	}
}

// Seen in the spec 006 verification: a band that straddles a hard income
// threshold was explained as the agency's decision. The searcher's exact
// income decides it, and only a discretionary rule is the agency's call.
func TestTemplateNamesWhoDecidesAConditionalVerdict(t *testing.T) {
	triple := []eligibility.Rule{{Fact: "income_band", Operator: "income_multiple", Values: []string{"3"}, Hardness: "hard", Evidence: "ingresos que tripliquen el alquiler"}}
	inventory := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("income", "palermo", 500000, triple),
		candidate("owner", "palermo", 600000, ownerDecides),
	}}}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, failingWriter{})
	resp := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"caucion"}, "income_band": {"1000000-2000000"}})

	lines := map[string]string{}
	for _, line := range strings.Split(resp.Reply, "\n") {
		for _, evidence := range []string{triple[0].Evidence, ownerDecides[0].Evidence} {
			if strings.Contains(line, evidence) {
				lines[evidence] = line
			}
		}
	}
	if l := lines[triple[0].Evidence]; !strings.Contains(l, "depende de tus ingresos") || strings.Contains(l, "inmobiliaria") {
		t.Errorf("a near-line income rule depends on the searcher's income, got %q", l)
	}
	if l := lines[ownerDecides[0].Evidence]; !strings.Contains(l, "depende de la inmobiliaria") {
		t.Errorf("a discretionary rule is the agency's call, got %q", l)
	}
}

// Seen in the spec 006 verification: a searcher who declared no guarantee was
// told "no tenemos información sobre tu garantía". The model never saw what
// the searcher declared, from the form or the chat.
func TestTheModelSeesWhatTheSearcherDeclared(t *testing.T) {
	inventory := stock{byHood: map[string][]eligibility.Candidate{"palermo": {candidate("caucion", "palermo", 700000, caucionOnly)}}}
	model := &recordingLLM{}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, buyer.LocalWriter{Client: model})
	turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"none"}})

	packet := model.last.Messages[len(model.last.Messages)-1].Content
	if !strings.Contains(packet, `"declared":{"guarantee":["none"]}`) {
		t.Fatalf("the model's packet must carry the declared qualification, got:\n%s", packet)
	}
}

// Seen live: with one listing hidden, the reply counted it twice ("encontré
// 1 departamento" and "hay además 1 propiedad oculta"). The model gets the
// hidden count once, as a sentence it can say as is.
func TestTheModelLearnsTheHiddenCountOnce(t *testing.T) {
	inventory := stock{byHood: map[string][]eligibility.Candidate{"palermo": {candidate("caucion", "palermo", 700000, caucionOnly)}}}
	model := &recordingLLM{}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, buyer.LocalWriter{Client: model})
	turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})

	packet := model.last.Messages[len(model.last.Messages)-1].Content
	if !strings.Contains(packet, `"hidden_note":"1 propiedad no se muestra porque no cumplís sus requisitos."`) || strings.Contains(packet, "hidden_ineligible") {
		t.Fatalf("want the hidden count once, as a sentence, got:\n%s", packet)
	}
}
