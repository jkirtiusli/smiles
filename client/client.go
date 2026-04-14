package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"smiles/model"
	"sort"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	searchHost = "api-air-flightsearch-green.smiles.com.ar"
	taxHost    = "api-airlines-boarding-tax-prd.smiles.com.br"
	dateLayout = "2006-01-02"
)

// SearchParams holds parameters for a single-date flight search.
type SearchParams struct {
	Origin      string
	Destination string
	Date        time.Time
	Adults      int
	CabinType   string
	Currency    string
}

// RoundTripParams holds parameters for a multi-day cheapest flight search.
type RoundTripParams struct {
	Origin        string
	Destination   string
	DepartureDate time.Time
	ReturnDate    time.Time
	DaysToQuery   int
	FareType      string
	OneWay        bool
}

// CheapestFlight represents the cheapest flight found for a given date.
type CheapestFlight struct {
	Flight *model.Flight
	Fare   *model.Fare
	Date   time.Time
}

// CheapestResult holds the results of a multi-day cheapest flight search.
type CheapestResult struct {
	OutboundPerDay   []CheapestFlight
	OutboundCheapest *CheapestFlight
	ReturnPerDay     []CheapestFlight
	ReturnCheapest   *CheapestFlight
	OutboundTax      *model.BoardingTax
	ReturnTax        *model.BoardingTax
}

// SmilesClient wraps HTTP interactions with the Smiles API.
type SmilesClient struct {
	// tlsClient is used in production for Chrome TLS+HTTP/2 impersonation.
	tlsClient tls_client.HttpClient
	// httpClient is used as fallback (tests with httptest).
	httpClient  *http.Client
	apiKey      string
	bearerToken string
	region      string
	origin      string
}

// New creates a new SmilesClient with Chrome browser impersonation.
func New(apiKey, bearerToken string) *SmilesClient {
	jar := tls_client.NewCookieJar()
	tlsClient, _ := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_124),
		tls_client.WithCookieJar(jar),
	)
	return &SmilesClient{
		tlsClient:   tlsClient,
		apiKey:      apiKey,
		bearerToken: bearerToken,
		region:      "ARGENTINA",
		origin:      "https://www.smiles.com.ar",
	}
}

// SearchFlights searches for flights on a specific date.
func (sc *SmilesClient) SearchFlights(ctx context.Context, params SearchParams) (*model.Data, error) {
	if params.Adults == 0 {
		params.Adults = 1
	}
	if params.CabinType == "" {
		params.CabinType = "all"
	}
	if params.Currency == "" {
		params.Currency = "ARS"
	}

	u := sc.buildSearchURL(params)
	var data model.Data
	if err := sc.doRequest(ctx, u, searchHost, &data); err != nil {
		return nil, fmt.Errorf("search flights %s->%s on %s: %w",
			params.Origin, params.Destination, params.Date.Format(dateLayout), err)
	}
	return &data, nil
}

// GetBoardingTax fetches boarding tax for a flight+fare combination.
func (sc *SmilesClient) GetBoardingTax(ctx context.Context, flightUID, fareUID string) (*model.BoardingTax, error) {
	u := sc.buildTaxURL(flightUID, fareUID)
	var data model.BoardingTax
	if err := sc.doRequest(ctx, u, taxHost, &data); err != nil {
		return nil, fmt.Errorf("get boarding tax for flight %s: %w", flightUID, err)
	}
	return &data, nil
}

// FindCheapestFlights searches across multiple days and returns the cheapest options.
func (sc *SmilesClient) FindCheapestFlights(ctx context.Context, params RoundTripParams) (*CheapestResult, error) {
	if params.FareType == "" {
		params.FareType = "SMILES_CLUB"
	}
	if params.DaysToQuery <= 0 {
		params.DaysToQuery = 1
	}
	if params.DaysToQuery > 31 {
		params.DaysToQuery = 31
	}

	// Limit concurrency to avoid API rate-limiting (max 3 concurrent requests).
	sem := make(chan struct{}, 3)

	type searchResult struct {
		result model.Result
		err    error
	}

	departuresCh := make(chan searchResult, params.DaysToQuery)
	var returnsCh chan searchResult
	if !params.OneWay {
		returnsCh = make(chan searchResult, params.DaysToQuery)
	}

	var wg sync.WaitGroup

	for i := 0; i < params.DaysToQuery; i++ {
		depDate := params.DepartureDate.AddDate(0, 0, i)

		wg.Add(1)
		go func(date time.Time) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, err := sc.SearchFlights(ctx, SearchParams{
				Origin:      params.Origin,
				Destination: params.Destination,
				Date:        date,
			})
			if err != nil {
				departuresCh <- searchResult{err: err}
				return
			}
			departuresCh <- searchResult{result: model.Result{Data: *data, QueryDate: date}}
		}(depDate)

		if !params.OneWay {
			retDate := params.ReturnDate.AddDate(0, 0, i)
			wg.Add(1)
			go func(date time.Time) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				data, err := sc.SearchFlights(ctx, SearchParams{
					Origin:      params.Destination,
					Destination: params.Origin,
					Date:        date,
				})
				if err != nil {
					returnsCh <- searchResult{err: err}
					return
				}
				returnsCh <- searchResult{result: model.Result{Data: *data, QueryDate: date}}
			}(retDate)
		}
	}

	wg.Wait()
	close(departuresCh)
	if returnsCh != nil {
		close(returnsCh)
	}

	var departureResults []model.Result
	var returnResults []model.Result

	for sr := range departuresCh {
		if sr.err != nil {
			continue
		}
		departureResults = append(departureResults, sr.result)
	}
	if returnsCh != nil {
		for sr := range returnsCh {
			if sr.err != nil {
				continue
			}
			returnResults = append(returnResults, sr.result)
		}
	}

	if len(departureResults) == 0 && len(returnResults) == 0 {
		return nil, fmt.Errorf("all searches failed, API may be rate-limiting")
	}

	sortResults(departureResults)
	sortResults(returnResults)

	result := &CheapestResult{}
	result.OutboundPerDay, result.OutboundCheapest = findCheapest(departureResults, params.FareType)
	result.ReturnPerDay, result.ReturnCheapest = findCheapest(returnResults, params.FareType)

	return result, nil
}

// doRequest executes an HTTP GET and unmarshals the JSON response.
// Uses tls-client (Chrome impersonation) in production, standard http.Client in tests.
func (sc *SmilesClient) doRequest(ctx context.Context, u url.URL, authority string, out interface{}) error {
	var statusCode int
	var body []byte
	var err error

	if sc.httpClient != nil {
		statusCode, body, err = sc.doStandardRequest(ctx, u.String())
	} else {
		statusCode, body, err = sc.doChromeRequest(u.String())
	}

	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	if statusCode != 200 {
		return fmt.Errorf("API returned status %d for %s", statusCode, u.String())
	}
	if len(body) == 0 {
		return fmt.Errorf("empty response body")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("unmarshalling response: %w", err)
	}
	return nil
}

// doChromeRequest uses tls-client with Chrome browser impersonation.
func (sc *SmilesClient) doChromeRequest(rawURL string) (int, []byte, error) {
	req, err := fhttp.NewRequest("GET", rawURL, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header = fhttp.Header{
		"accept":             {"application/json, text/plain, */*"},
		"accept-language":    {"es-AR,es-419;q=0.9,es;q=0.8,en;q=0.7"},
		"channel":            {"Mobile"},
		"language":           {"es-ES"},
		"origin":             {sc.origin},
		"priority":           {"u=1, i"},
		"referer":            {sc.origin + "/"},
		"region":             {sc.region},
		"sec-ch-ua":          {`"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`},
		"sec-ch-ua-mobile":   {"?1"},
		"sec-ch-ua-platform": {`"Android"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-site"},
		"user-agent":         {"Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36"},
		"x-api-key":          {sc.apiKey},
		fhttp.HeaderOrderKey: {
			"accept", "accept-language", "channel", "language",
			"origin", "priority", "referer", "region",
			"sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
			"sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
			"user-agent", "x-api-key",
		},
	}

	if sc.bearerToken != "" {
		req.Header.Set("authorization", "Bearer "+sc.bearerToken)
	}

	res, err := sc.tlsClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, body, nil
}

// doStandardRequest uses standard net/http (for tests with httptest).
func (sc *SmilesClient) doStandardRequest(ctx context.Context, rawURL string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("x-api-key", sc.apiKey)
	req.Header.Set("region", sc.region)
	req.Header.Set("origin", sc.origin)

	res, err := sc.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, body, nil
}

func (sc *SmilesClient) buildSearchURL(params SearchParams) url.URL {
	u := url.URL{
		Scheme: "https",
		Host:   searchHost,
		Path:   "/v1/airlines/search",
	}
	q := url.Values{}
	q.Set("adults", fmt.Sprintf("%d", params.Adults))
	q.Set("cabinType", params.CabinType)
	q.Set("children", "0")
	q.Set("currencyCode", params.Currency)
	q.Set("infants", "0")
	q.Set("isFlexibleDateChecked", "false")
	q.Set("tripType", "2")
	q.Set("forceCongener", "true")
	q.Set("r", "ar")
	q.Set("departureDate", params.Date.Format(dateLayout))
	q.Set("originAirportCode", params.Origin)
	q.Set("destinationAirportCode", params.Destination)
	u.RawQuery = q.Encode()
	return u
}

func (sc *SmilesClient) buildTaxURL(flightUID, fareUID string) url.URL {
	u := url.URL{
		Scheme: "https",
		Host:   taxHost,
		Path:   "/v1/airlines/flight/boardingtax",
	}
	q := url.Values{}
	q.Set("adults", "1")
	q.Set("children", "0")
	q.Set("infants", "0")
	q.Set("highlightText", "SMILES_CLUB")
	q.Set("type", "SEGMENT_1")
	q.Set("uid", flightUID)
	q.Set("fareuid", fareUID)
	u.RawQuery = q.Encode()
	return u
}

// GetFareByType returns the first fare matching the given type, or nil if not found.
func GetFareByType(f *model.Flight, fareType string) *model.Fare {
	for i, v := range f.FareList {
		if v.FType == fareType {
			return &f.FareList[i]
		}
	}
	return nil
}

func findCheapest(results []model.Result, fareType string) ([]CheapestFlight, *CheapestFlight) {
	var perDay []CheapestFlight
	var overallCheapest *CheapestFlight

	for _, r := range results {
		if len(r.Data.RequestedFlightSegmentList) == 0 {
			continue
		}

		var bestFlight *model.Flight
		var bestFare *model.Fare

		for i := range r.Data.RequestedFlightSegmentList[0].FlightList {
			f := &r.Data.RequestedFlightSegmentList[0].FlightList[i]
			fare := GetFareByType(f, fareType)
			if fare == nil {
				continue
			}
			if bestFare == nil || fare.Miles < bestFare.Miles {
				bestFlight = f
				bestFare = fare
			}
		}

		if bestFlight != nil {
			cf := CheapestFlight{
				Flight: bestFlight,
				Fare:   bestFare,
				Date:   r.QueryDate,
			}
			perDay = append(perDay, cf)

			if overallCheapest == nil || bestFare.Miles < overallCheapest.Fare.Miles {
				overallCheapest = &cf
			}
		}
	}

	return perDay, overallCheapest
}

func sortResults(r []model.Result) {
	sort.Slice(r, func(i, j int) bool {
		return r[i].QueryDate.Before(r[j].QueryDate)
	})
}
