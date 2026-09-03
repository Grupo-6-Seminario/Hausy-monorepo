package buyer_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// toolCallingClient replays scripted responses and records what it was offered.
type toolCallingClient struct {
	replies  []llm.ChatResponse
	requests []llm.ChatRequest
}

func (c *toolCallingClient) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.requests = append(c.requests, req)
	index := len(c.requests) - 1
	if index >= len(c.replies) {
		return &llm.ChatResponse{Content: "Listo."}, nil
	}
	reply := c.replies[index]
	return &reply, nil
}

type fakeInventory struct {
	results search.Results
	details map[string]listing.Listing
	queries []search.Query
}

func (f *fakeInventory) Search(_ context.Context, query search.Query) (search.Results, error) {
	f.queries = append(f.queries, query)
	return f.results, nil
}

func (f *fakeInventory) ByURL(_ context.Context, url string) (listing.Listing, error) {
	return f.details[url], nil
}

func (f *fakeInventory) Neighborhoods(context.Context) ([]search.Neighborhood, error) {
	return []search.Neighborhood{{Slug: "palermo", Listings: 2}}, nil
}

func (f *fakeInventory) PriceStats(context.Context, search.StatsQuery) (search.Stats, error) {
	return search.Stats{}, nil
}

func searchCall(arguments string) llm.ChatResponse {
	return llm.ChatResponse{ToolCalls: []llm.ToolCall{{
		ID: "call_1", Name: "search_listings", Arguments: json.RawMessage(arguments),
	}}}
}

func inventoryWithTwoListings() *fakeInventory {
	return &fakeInventory{
		results: search.Results{TotalMatches: 2, Matches: []search.Match{
			{URL: "https://ex.test/1", Neighborhood: "palermo"},
			{URL: "https://ex.test/2", Neighborhood: "palermo"},
		}},
		details: map[string]listing.Listing{
			"https://ex.test/1": {URL: "https://ex.test/1", Neighborhood: "palermo", Description: "Luminoso."},
			"https://ex.test/2": {URL: "https://ex.test/2", Neighborhood: "palermo", Description: "Contrafrente."},
		},
	}
}

func TestAgent_WithInventory_OffersTheSearchToolsToTheModel(t *testing.T) {
	client := &toolCallingClient{replies: []llm.ChatResponse{{Content: "¿En qué zona?"}}}
	agent := buyer.NewAgent(client, buyer.WithInventory(inventoryWithTwoListings()))

	if _, err := agent.HandleMessage(context.Background(), "s1", "Hola"); err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	offered := map[string]bool{}
	for _, definition := range client.requests[0].Tools {
		offered[definition.Name] = true
	}
	for _, name := range []string{"list_neighborhoods", "search_listings", "get_listing", "neighborhood_price_stats"} {
		if !offered[name] {
			t.Errorf("expected %q to be offered to the model, got %v", name, offered)
		}
	}
	if !strings.Contains(client.requests[0].Messages[0].Content, "search_listings") {
		t.Error("expected the system prompt to tell the model how to use the tools")
	}
}

// The cards the user sees come from the search that actually ran, not from
// URLs re-typed by the model into its prose.
func TestAgent_WithInventory_ReturnsTheListingsTheSearchActuallyFound(t *testing.T) {
	inventory := inventoryWithTwoListings()
	client := &toolCallingClient{replies: []llm.ChatResponse{
		searchCall(`{"neighborhoods":["palermo"],"operation":"alquiler"}`),
		{Content: "Encontré dos opciones en Palermo."},
	}}
	agent := buyer.NewAgent(client, buyer.WithInventory(inventory))

	response, err := agent.HandleMessage(context.Background(), "s1", "Alquiler en Palermo")
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if response.Reply != "Encontré dos opciones en Palermo." {
		t.Errorf("expected the model's final reply, got %q", response.Reply)
	}
	if len(response.Listings) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(response.Listings))
	}
	if response.Listings[0].URL != "https://ex.test/1" || response.Listings[0].Description != "Luminoso." {
		t.Errorf("expected the full stored listing, got %+v", response.Listings[0])
	}
}

// A second, narrower search supersedes the first: the user is shown what the
// agent settled on, not everything it looked at along the way.
func TestAgent_WithInventory_ShowsTheLastSearchWhenTheModelNarrows(t *testing.T) {
	inventory := inventoryWithTwoListings()
	client := &toolCallingClient{replies: []llm.ChatResponse{
		searchCall(`{"neighborhoods":["palermo"]}`),
		searchCall(`{"neighborhoods":["palermo"],"min_bedrooms":2}`),
		{Content: "Con dos dormitorios quedan estas."},
	}}
	agent := buyer.NewAgent(client, buyer.WithInventory(inventory))

	response, err := agent.HandleMessage(context.Background(), "s1", "Dos dormitorios en Palermo")
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if len(inventory.queries) != 2 {
		t.Fatalf("expected both searches to run, got %d", len(inventory.queries))
	}

	// Requirements describe the query that produced what is on screen.
	found := false
	for _, requirement := range response.Requirements {
		if requirement.Type == "min_bedrooms" && requirement.Value == "2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the narrowed query to be reported as a requirement, got %+v", response.Requirements)
	}
}

func TestAgent_WithInventory_CarriesTheConversationIntoTheNextTurn(t *testing.T) {
	client := &toolCallingClient{replies: []llm.ChatResponse{
		{Content: "¿En qué zona?"},
		{Content: "Busco en Palermo entonces."},
	}}
	agent := buyer.NewAgent(client, buyer.WithInventory(inventoryWithTwoListings()))
	ctx := context.Background()

	if _, err := agent.HandleMessage(ctx, "s1", "Busco depto"); err != nil {
		t.Fatalf("turn 1 failed: %v", err)
	}
	if _, err := agent.HandleMessage(ctx, "s1", "Palermo"); err != nil {
		t.Fatalf("turn 2 failed: %v", err)
	}

	second := client.requests[1].Messages
	if len(second) < 4 {
		t.Fatalf("expected system + turn 1 + turn 2 in the second request, got %d messages", len(second))
	}
	if second[len(second)-1].Content != "Palermo" {
		t.Errorf("expected the new message last, got %q", second[len(second)-1].Content)
	}
	if second[len(second)-2].Content != "¿En qué zona?" {
		t.Errorf("expected the previous reply to be remembered, got %q", second[len(second)-2].Content)
	}
}

func TestAgent_WithInventory_KeepsSessionsApart(t *testing.T) {
	client := &toolCallingClient{}
	agent := buyer.NewAgent(client, buyer.WithInventory(inventoryWithTwoListings()))
	ctx := context.Background()

	if _, err := agent.HandleMessage(ctx, "s1", "Palermo"); err != nil {
		t.Fatalf("session 1 failed: %v", err)
	}
	if _, err := agent.HandleMessage(ctx, "s2", "Congreso"); err != nil {
		t.Fatalf("session 2 failed: %v", err)
	}

	for _, message := range client.requests[1].Messages {
		if message.Content == "Palermo" {
			t.Fatal("one session's conversation leaked into another")
		}
	}
}
