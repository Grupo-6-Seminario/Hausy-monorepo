package buyer_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

type failingWriter struct{}

func (failingWriter) Write(context.Context, buyer.Packet) (string, error) {
	return "", errors.New("local model down")
}

func TestAFailedWriterStillExplainsTheRanking(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), failingWriter{}))
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

func TestLocalWriterExplainsFromThePacketInOneCallWithoutTools(t *testing.T) {
	client := &recordingLLM{}
	w := &fakeWriter{}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), w))
	turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})

	reply, err := buyer.LocalWriter{Client: client}.Write(context.Background(), w.packets[0])
	if err != nil || reply != "## Mi lectura\n#1 es la mejor." {
		t.Fatalf("got %q, %v", reply, err)
	}
	prompt := client.last.Messages[len(client.last.Messages)-1].Content
	if len(client.last.Tools) != 0 || !strings.Contains(prompt, `"rank":1`) || !strings.Contains(prompt, "ver cuáles permite la propietaria") {
		t.Fatalf("the writer must receive the packet and no tools: %+v", client.last)
	}
}
