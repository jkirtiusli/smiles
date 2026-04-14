package model

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlexFloat64 handles JSON values that can be either a number or a string
// containing a number (the Smiles API is inconsistent about this).
type FlexFloat64 float64

func (f *FlexFloat64) UnmarshalJSON(data []byte) error {
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexFloat64(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = FlexFloat64(n)
	return nil
}

type Fare struct {
	UId             string      `json:"uid"`
	FType           string      `json:"type"`
	Miles           int         `json:"miles"`
	BaseMiles       int         `json:"baseMiles"`
	Money           float64     `json:"money"`
	AirlineFareAmt  FlexFloat64 `json:"airlineFareAmount"`
	AirlineTax      FlexFloat64 `json:"airlineTax"`
	FareValue       float64     `json:"fareValue"`
	LegListCost     string      `json:"legListCost"`
	LegListCurrency string      `json:"legListCurrency"`
	Offer           int         `json:"offer"`
}

type Airline struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type FlightDetail struct {
	Date    time.Time `json:"date"`
	Airport Airport   `json:"airport"`
}

type Duration struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

type Leg struct {
	Cabin     string       `json:"cabin"`
	Departure FlightDetail `json:"departure"`
	Arrival   FlightDetail `json:"arrival"`
}

type Flight struct {
	UId             string       `json:"uid"`
	Cabin           string       `json:"cabin"`
	Stops           int          `json:"stops"`
	AvailableSeats  int          `json:"availableSeats"`
	Duration        Duration     `json:"duration"`
	DurationNumber  int          `json:"durationNumber"`
	SourceFare      string       `json:"sourceFare"`
	AirportMainStop Airport      `json:"airportMainStop"`
	TimeStop        Duration     `json:"timeStop"`
	HourMainStop    string       `json:"hourMainStop"`
	Departure       FlightDetail `json:"departure"`
	Arrival         FlightDetail `json:"arrival"`
	Airline         Airline      `json:"airline"`
	LegList         []Leg        `json:"legList"`
	FareList        []Fare       `json:"fareList"`
}

type BestPricing struct {
	Miles      int    `json:"miles"`
	SourceFare string `json:"sourceFare"`
	Fare       Fare   `json:"fare"`
}

type Segment struct {
	SegmentType string      `json:"type"`
	FlightList  []Flight    `json:"flightList"`
	BestPricing BestPricing `json:"bestPricing"`
	Airports    Airports    `json:"airports"`
}

type Airport struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type Airports struct {
	DepartureAirports []Airport `json:"departureAirportList"`
	ArrivalAirports   []Airport `json:"arrivalAirportList"`
}

type Data struct {
	RequestedFlightSegmentList []Segment `json:"requestedFlightSegmentList"`
}

type Result struct {
	Data      Data
	QueryDate time.Time
}

type Totals struct {
	Total     Total `json:"total"`
	TotalFare Total `json:"totalFare"`
}

type Total struct {
	Miles int     `json:"miles"`
	Money float64 `json:"money"`
}

type BoardingTax struct {
	Totals Totals `json:"totals"`
}

// needed because the date expected has the format "2006-01-02T15:04:05"
func (f *FlightDetail) UnmarshalJSON(p []byte) error {
	var aux struct {
		Date    string  `json:"date"`
		Airport Airport `json:"airport"`
	}

	err := json.Unmarshal(p, &aux)
	if err != nil {
		return err
	}

	t, err := time.Parse("2006-01-02T15:04:05", aux.Date)
	if err != nil {
		return err
	}

	f.Date = t
	f.Airport = aux.Airport

	return nil
}
