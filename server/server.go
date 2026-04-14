package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"smiles/client"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const dateLayout = "2006-01-02"

// NewSmilesServer creates an MCP server with flight search tools.
func NewSmilesServer(sc *client.SmilesClient) *server.MCPServer {
	s := server.NewMCPServer(
		"smiles-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	s.AddTool(searchFlightsTool(), makeSearchFlightsHandler(sc))
	s.AddTool(findCheapestFlightsTool(), makeFindCheapestFlightsHandler(sc))
	s.AddTool(getFlightTaxesTool(), makeGetFlightTaxesHandler(sc))

	return s
}

// --- Tool Definitions ---

func searchFlightsTool() mcp.Tool {
	return mcp.NewTool("search_flights",
		mcp.WithDescription("Search for Smiles miles flights between two airports on a specific date. Returns all available flights with pricing in miles (SMILES, SMILES_CLUB, SMILES_MONEY fare types), airline, stops, duration, and available seats."),
		mcp.WithString("origin",
			mcp.Required(),
			mcp.Description("Origin airport IATA code (e.g. EZE, GRU, GIG)"),
		),
		mcp.WithString("destination",
			mcp.Required(),
			mcp.Description("Destination airport IATA code (e.g. PUJ, MIA, SCL)"),
		),
		mcp.WithString("departure_date",
			mcp.Required(),
			mcp.Description("Departure date in YYYY-MM-DD format"),
		),
		mcp.WithString("cabin",
			mcp.Description("Cabin type filter: all, ECONOMIC, BUSINESS. Default: all"),
		),
		mcp.WithNumber("adults",
			mcp.Description("Number of adult passengers. Default: 1"),
		),
	)
}

func findCheapestFlightsTool() mcp.Tool {
	return mcp.NewTool("find_cheapest_flights",
		mcp.WithDescription("Find the cheapest roundtrip flights in miles across a date range. Searches multiple consecutive days concurrently and returns the cheapest option per day plus the overall cheapest for both outbound and return legs."),
		mcp.WithString("origin",
			mcp.Required(),
			mcp.Description("Origin airport IATA code (e.g. EZE, GRU)"),
		),
		mcp.WithString("destination",
			mcp.Required(),
			mcp.Description("Destination airport IATA code (e.g. PUJ, MIA)"),
		),
		mcp.WithString("departure_date",
			mcp.Required(),
			mcp.Description("First departure date to search (YYYY-MM-DD)"),
		),
		mcp.WithString("return_date",
			mcp.Required(),
			mcp.Description("First return date to search (YYYY-MM-DD)"),
		),
		mcp.WithNumber("days",
			mcp.Description("Number of consecutive days to search from each start date. Default: 1, Max: 10"),
		),
		mcp.WithString("fare_type",
			mcp.Description("Fare type to compare: SMILES, SMILES_CLUB, SMILES_MONEY, SMILES_MONEY_CLUB. Default: SMILES_CLUB"),
		),
	)
}

func getFlightTaxesTool() mcp.Tool {
	return mcp.NewTool("get_flight_taxes",
		mcp.WithDescription("Get boarding taxes and fees for a specific flight and fare combination. Use flight_uid and fare_uid from search_flights results."),
		mcp.WithString("flight_uid",
			mcp.Required(),
			mcp.Description("Flight UID from search_flights results"),
		),
		mcp.WithString("fare_uid",
			mcp.Required(),
			mcp.Description("Fare UID from search_flights results"),
		),
	)
}

// --- Handlers ---

func makeSearchFlightsHandler(sc *client.SmilesClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		origin := mcp.ParseString(request, "origin", "")
		if origin == "" {
			return mcp.NewToolResultError("origin is required"), nil
		}
		destination := mcp.ParseString(request, "destination", "")
		if destination == "" {
			return mcp.NewToolResultError("destination is required"), nil
		}
		dateStr := mcp.ParseString(request, "departure_date", "")
		if dateStr == "" {
			return mcp.NewToolResultError("departure_date is required"), nil
		}

		date, err := time.Parse(dateLayout, dateStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid date format %q, expected YYYY-MM-DD", dateStr)), nil
		}

		cabin := mcp.ParseString(request, "cabin", "all")
		adults := mcp.ParseInt(request, "adults", 1)

		data, err := sc.SearchFlights(ctx, client.SearchParams{
			Origin:      origin,
			Destination: destination,
			Date:        date,
			Adults:      adults,
			CabinType:   cabin,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		result, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshalling results: %w", err)
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}

func makeFindCheapestFlightsHandler(sc *client.SmilesClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		origin := mcp.ParseString(request, "origin", "")
		if origin == "" {
			return mcp.NewToolResultError("origin is required"), nil
		}
		destination := mcp.ParseString(request, "destination", "")
		if destination == "" {
			return mcp.NewToolResultError("destination is required"), nil
		}
		depDateStr := mcp.ParseString(request, "departure_date", "")
		if depDateStr == "" {
			return mcp.NewToolResultError("departure_date is required"), nil
		}
		retDateStr := mcp.ParseString(request, "return_date", "")
		if retDateStr == "" {
			return mcp.NewToolResultError("return_date is required"), nil
		}

		depDate, err := time.Parse(dateLayout, depDateStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid departure_date format %q", depDateStr)), nil
		}
		retDate, err := time.Parse(dateLayout, retDateStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid return_date format %q", retDateStr)), nil
		}
		if retDate.Before(depDate) {
			return mcp.NewToolResultError("return_date must be after departure_date"), nil
		}

		days := mcp.ParseInt(request, "days", 1)
		fareType := mcp.ParseString(request, "fare_type", "SMILES_CLUB")

		cheapest, err := sc.FindCheapestFlights(ctx, client.RoundTripParams{
			Origin:        origin,
			Destination:   destination,
			DepartureDate: depDate,
			ReturnDate:    retDate,
			DaysToQuery:   days,
			FareType:      fareType,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		type flightSummary struct {
			Date     string `json:"date"`
			Origin   string `json:"origin"`
			Dest     string `json:"destination"`
			Airline  string `json:"airline"`
			Cabin    string `json:"cabin"`
			Stops    int    `json:"stops"`
			Miles    int    `json:"miles"`
			FareType string `json:"fare_type"`
		}

		type response struct {
			OutboundPerDay   []flightSummary `json:"outbound_per_day"`
			OutboundCheapest *flightSummary  `json:"outbound_cheapest"`
			ReturnPerDay     []flightSummary `json:"return_per_day"`
			ReturnCheapest   *flightSummary  `json:"return_cheapest"`
		}

		toSummary := func(cf client.CheapestFlight) flightSummary {
			return flightSummary{
				Date:     cf.Flight.Departure.Date.Format(dateLayout),
				Origin:   cf.Flight.Departure.Airport.Code,
				Dest:     cf.Flight.Arrival.Airport.Code,
				Airline:  cf.Flight.Airline.Name,
				Cabin:    cf.Flight.Cabin,
				Stops:    cf.Flight.Stops,
				Miles:    cf.Fare.Miles,
				FareType: cf.Fare.FType,
			}
		}

		resp := response{}
		for _, cf := range cheapest.OutboundPerDay {
			resp.OutboundPerDay = append(resp.OutboundPerDay, toSummary(cf))
		}
		for _, cf := range cheapest.ReturnPerDay {
			resp.ReturnPerDay = append(resp.ReturnPerDay, toSummary(cf))
		}
		if cheapest.OutboundCheapest != nil {
			s := toSummary(*cheapest.OutboundCheapest)
			resp.OutboundCheapest = &s
		}
		if cheapest.ReturnCheapest != nil {
			s := toSummary(*cheapest.ReturnCheapest)
			resp.ReturnCheapest = &s
		}

		result, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshalling results: %w", err)
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}

func makeGetFlightTaxesHandler(sc *client.SmilesClient) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		flightUID := mcp.ParseString(request, "flight_uid", "")
		if flightUID == "" {
			return mcp.NewToolResultError("flight_uid is required"), nil
		}
		fareUID := mcp.ParseString(request, "fare_uid", "")
		if fareUID == "" {
			return mcp.NewToolResultError("fare_uid is required"), nil
		}

		tax, err := sc.GetBoardingTax(ctx, flightUID, fareUID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get taxes: %v", err)), nil
		}

		result, err := json.MarshalIndent(tax, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshalling tax result: %w", err)
		}

		return mcp.NewToolResultText(string(result)), nil
	}
}
