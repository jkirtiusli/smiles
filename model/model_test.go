package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFlightDetailUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDate time.Time
		wantCode string
		wantErr  bool
	}{
		{
			name:     "valid flight detail",
			input:    `{"date":"2023-02-08T07:45:00","airport":{"code":"EZE","name":"Ministro Pistarini","city":"Buenos Aires","country":"Argentina"}}`,
			wantDate: time.Date(2023, 2, 8, 7, 45, 0, 0, time.UTC),
			wantCode: "EZE",
		},
		{
			name:    "invalid date format",
			input:   `{"date":"2023/02/08","airport":{"code":"EZE"}}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fd FlightDetail
			err := json.Unmarshal([]byte(tt.input), &fd)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !fd.Date.Equal(tt.wantDate) {
				t.Errorf("date = %v, want %v", fd.Date, tt.wantDate)
			}
			if fd.Airport.Code != tt.wantCode {
				t.Errorf("airport code = %q, want %q", fd.Airport.Code, tt.wantCode)
			}
		})
	}
}

func TestFareUnmarshal(t *testing.T) {
	input := `{
		"uid": "abc123",
		"type": "SMILES_CLUB",
		"miles": 82000,
		"baseMiles": 90000,
		"money": 0,
		"airlineFareAmount": 315.76,
		"airlineTax": 116.30,
		"fareValue": 0.00630,
		"legListCost": "EZE-BOG = 233.04 / BOG-PUJ = 82.72",
		"legListCurrency": "USD",
		"offer": 1
	}`

	var fare Fare
	if err := json.Unmarshal([]byte(input), &fare); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fare.UId != "abc123" {
		t.Errorf("uid = %q, want %q", fare.UId, "abc123")
	}
	if fare.FType != "SMILES_CLUB" {
		t.Errorf("type = %q, want %q", fare.FType, "SMILES_CLUB")
	}
	if fare.Miles != 82000 {
		t.Errorf("miles = %d, want %d", fare.Miles, 82000)
	}
	if fare.BaseMiles != 90000 {
		t.Errorf("baseMiles = %d, want %d", fare.BaseMiles, 90000)
	}
	if float64(fare.AirlineFareAmt) != 315.76 {
		t.Errorf("airlineFareAmount = %f, want %f", float64(fare.AirlineFareAmt), 315.76)
	}
	if float64(fare.AirlineTax) != 116.30 {
		t.Errorf("airlineTax = %f, want %f", float64(fare.AirlineTax), 116.30)
	}
	if fare.LegListCost != "EZE-BOG = 233.04 / BOG-PUJ = 82.72" {
		t.Errorf("legListCost = %q, want expected value", fare.LegListCost)
	}
}

func TestFlightUnmarshalWithNewFields(t *testing.T) {
	input := `{
		"uid": "flight1",
		"cabin": "BUSINESS",
		"stops": 1,
		"availableSeats": 3,
		"duration": {"hours": 10, "minutes": 15},
		"durationNumber": 1015,
		"sourceFare": "AWARD",
		"airportMainStop": {"code": "BOG", "name": "El Dorado", "city": "Bogota", "country": "Colombia"},
		"timeStop": {"hours": 1, "minutes": 15},
		"hourMainStop": "12:00-13:15",
		"departure": {"date": "2023-02-08T07:45:00", "airport": {"code": "EZE"}},
		"arrival": {"date": "2023-02-08T17:00:00", "airport": {"code": "PUJ"}},
		"airline": {"code": "AV", "name": "Avianca"},
		"fareList": [
			{"uid": "f1", "type": "SMILES_CLUB", "miles": 82000}
		]
	}`

	var flight Flight
	if err := json.Unmarshal([]byte(input), &flight); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if flight.AvailableSeats != 3 {
		t.Errorf("availableSeats = %d, want 3", flight.AvailableSeats)
	}
	if flight.Duration.Hours != 10 || flight.Duration.Minutes != 15 {
		t.Errorf("duration = %dh%dm, want 10h15m", flight.Duration.Hours, flight.Duration.Minutes)
	}
	if flight.AirportMainStop.Code != "BOG" {
		t.Errorf("airportMainStop.code = %q, want %q", flight.AirportMainStop.Code, "BOG")
	}
	if flight.TimeStop.Hours != 1 || flight.TimeStop.Minutes != 15 {
		t.Errorf("timeStop = %dh%dm, want 1h15m", flight.TimeStop.Hours, flight.TimeStop.Minutes)
	}
	if flight.HourMainStop != "12:00-13:15" {
		t.Errorf("hourMainStop = %q, want %q", flight.HourMainStop, "12:00-13:15")
	}
	if len(flight.FareList) != 1 || flight.FareList[0].Miles != 82000 {
		t.Errorf("fareList not parsed correctly")
	}
}
