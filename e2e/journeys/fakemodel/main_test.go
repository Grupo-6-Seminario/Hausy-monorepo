package main

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

// The real planner and writer call the fake through the real client, so a
// change to either prompt or to the wire format fails here, not mid-journey.
func TestAnswersThePlannerAndTheWriter(t *testing.T) {
	server := httptest.NewServer(handler())
	defer server.Close()
	client := local.NewClient(server.URL, "", "")

	plan, err := intake.Qwen{Client: client}.Plan(context.Background(), []string{"Hola", "Busco alquiler en Monserrat"}, eligibility.Catalog{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Branches) != 1 || !slices.Equal(plan.Branches[0].Neighborhoods, []string{"monserrat"}) {
		t.Fatalf("plan = %+v, want one Monserrat branch", plan)
	}

	var streamed strings.Builder
	reply, err := buyer.LocalWriter{Client: client}.Write(context.Background(), buyer.Packet{}, func(delta string) { streamed.WriteString(delta) })
	if err != nil {
		t.Fatal(err)
	}
	if reply != writerReply || streamed.String() != writerReply {
		t.Fatalf("reply = %q, streamed = %q, want %q", reply, streamed.String(), writerReply)
	}
}

func TestRejectsAMessageWithNoCannedPlan(t *testing.T) {
	server := httptest.NewServer(handler())
	defer server.Close()

	_, err := intake.Qwen{Client: local.NewClient(server.URL, "", "")}.Plan(context.Background(), []string{"Busco en Narnia"}, eligibility.Catalog{})
	if err == nil || !strings.Contains(err.Error(), "status 500") || !strings.Contains(err.Error(), "Busco en Narnia") {
		t.Fatalf("err = %v, want a 500 that names the message", err)
	}
}
