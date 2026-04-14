package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"smiles/model"
	"testing"
	"time"
)

func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../data/response.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return data
}

func TestSearchFlights(t *testing.T) {
	fixture := loadFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("region") != "ARGENTINA" {
			t.Errorf("region header = %q, want ARGENTINA", r.Header.Get("region"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer server.Close()

	sc := &SmilesClient{
		httpClient: server.Client(),
		apiKey:     "test-key",
		region:     "ARGENTINA",
		origin:     "https://www.smiles.com.ar",
	}

	// Override the URL builder to point to our test server
	// We test doRequest directly since buildSearchURL generates external URLs
	ctx := context.Background()

	var data model.Data
	err := sc.doRequest(ctx, *mustParseURL(t, server.URL+"/v1/airlines/search"), "test-host", &data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.RequestedFlightSegmentList) == 0 {
		t.Fatal("expected flight segments, got none")
	}

	flights := data.RequestedFlightSegmentList[0].FlightList
	if len(flights) == 0 {
		t.Fatal("expected flights, got none")
	}

	// Verify enriched fields are parsed
	firstFlight := flights[0]
	if firstFlight.Cabin == "" {
		t.Error("cabin should not be empty")
	}
	if len(firstFlight.FareList) == 0 {
		t.Fatal("expected fares, got none")
	}

	// Check that enriched fare fields are parsed
	fare := firstFlight.FareList[0]
	if fare.FType == "" {
		t.Error("fare type should not be empty")
	}
	if fare.Miles == 0 {
		t.Error("fare miles should not be zero")
	}
}

func TestDoRequestReturnsErrorOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	sc := &SmilesClient{
		httpClient: server.Client(),
		apiKey:     "bad-key",
		region:     "ARGENTINA",
		origin:     "https://www.smiles.com.ar",
	}

	var data model.Data
	err := sc.doRequest(context.Background(), *mustParseURL(t, server.URL+"/test"), "test-host", &data)
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

func TestDoRequestReturnsErrorOnEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing
	}))
	defer server.Close()

	sc := &SmilesClient{
		httpClient: server.Client(),
		apiKey:     "test-key",
		region:     "ARGENTINA",
		origin:     "https://www.smiles.com.ar",
	}

	var data model.Data
	err := sc.doRequest(context.Background(), *mustParseURL(t, server.URL+"/test"), "test-host", &data)
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestGetFareByType(t *testing.T) {
	flight := &model.Flight{
		FareList: []model.Fare{
			{FType: "SMILES", Miles: 90000},
			{FType: "SMILES_CLUB", Miles: 82000},
			{FType: "SMILES_MONEY", Miles: 13500},
		},
	}

	fare := GetFareByType(flight, "SMILES_CLUB")
	if fare == nil {
		t.Fatal("expected fare, got nil")
	}
	if fare.Miles != 82000 {
		t.Errorf("miles = %d, want 82000", fare.Miles)
	}

	notFound := GetFareByType(flight, "NONEXISTENT")
	if notFound != nil {
		t.Error("expected nil for nonexistent fare type")
	}
}

func TestFindCheapest(t *testing.T) {
	results := []model.Result{
		{
			QueryDate: time.Date(2023, 2, 8, 0, 0, 0, 0, time.UTC),
			Data: model.Data{
				RequestedFlightSegmentList: []model.Segment{
					{
						FlightList: []model.Flight{
							{
								UId:   "f1",
								Cabin: "ECONOMIC",
								FareList: []model.Fare{
									{FType: "SMILES_CLUB", Miles: 50000},
								},
							},
							{
								UId:   "f2",
								Cabin: "ECONOMIC",
								FareList: []model.Fare{
									{FType: "SMILES_CLUB", Miles: 30000},
								},
							},
						},
					},
				},
			},
		},
		{
			QueryDate: time.Date(2023, 2, 9, 0, 0, 0, 0, time.UTC),
			Data: model.Data{
				RequestedFlightSegmentList: []model.Segment{
					{
						FlightList: []model.Flight{
							{
								UId:   "f3",
								Cabin: "ECONOMIC",
								FareList: []model.Fare{
									{FType: "SMILES_CLUB", Miles: 25000},
								},
							},
						},
					},
				},
			},
		},
	}

	perDay, cheapest := findCheapest(results, "SMILES_CLUB")

	if len(perDay) != 2 {
		t.Fatalf("perDay length = %d, want 2", len(perDay))
	}

	// Day 1 cheapest should be f2 (30000 miles)
	if perDay[0].Fare.Miles != 30000 {
		t.Errorf("day 1 cheapest miles = %d, want 30000", perDay[0].Fare.Miles)
	}

	// Day 2 cheapest should be f3 (25000 miles)
	if perDay[1].Fare.Miles != 25000 {
		t.Errorf("day 2 cheapest miles = %d, want 25000", perDay[1].Fare.Miles)
	}

	// Overall cheapest should be f3 (25000 miles)
	if cheapest == nil {
		t.Fatal("expected overall cheapest, got nil")
	}
	if cheapest.Fare.Miles != 25000 {
		t.Errorf("overall cheapest miles = %d, want 25000", cheapest.Fare.Miles)
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", rawURL, err)
	}
	return u
}
