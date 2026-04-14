package main

import (
	"context"
	"fmt"
	"os"
	"smiles/client"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

func main() {
	if len(os.Args) < 5 || len(os.Args) > 6 {
		fmt.Println("Forma de Uso:")
		fmt.Println("  Solo ida:    smiles EZE MAD,BCN,FCO 2026-06-01 30")
		fmt.Println("  Ida y vuelta: smiles EZE MAD,BCN 2026-06-01 2026-06-20 10")
		fmt.Println()
		fmt.Println("  Destinos separados por coma, hasta 31 días de búsqueda")
		os.Exit(1)
	}

	apiKey := os.Getenv("SMILES_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: la variable de entorno SMILES_API_KEY es requerida")
		os.Exit(1)
	}
	bearerToken := os.Getenv("SMILES_BEARER_TOKEN")

	origin := os.Args[1]
	if len(origin) != 3 {
		fmt.Fprintf(os.Stderr, "Error: el aeropuerto de origen %s no es válido (debe ser 3 letras)\n", origin)
		os.Exit(1)
	}

	destinations := strings.Split(os.Args[2], ",")
	for _, d := range destinations {
		if len(d) != 3 {
			fmt.Fprintf(os.Stderr, "Error: el aeropuerto de destino %s no es válido (debe ser 3 letras)\n", d)
			os.Exit(1)
		}
	}

	oneWay := len(os.Args) == 5
	params, err := parseArgs(os.Args[3:], oneWay)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sc := client.New(apiKey, bearerToken)
	ctx := context.Background()

	for i, dest := range destinations {
		if i > 0 {
			fmt.Println(strings.Repeat("═", 60))
		}
		searchDest(ctx, sc, origin, strings.ToUpper(dest), params, oneWay)
	}
}

func searchDest(ctx context.Context, sc *client.SmilesClient, origin, dest string, base client.RoundTripParams, oneWay bool) {
	p := base
	p.Origin = origin
	p.Destination = dest

	if oneWay {
		fmt.Printf("Buscando ida %s → %s (%s, %d días)\n",
			origin, dest, p.DepartureDate.Format(dateLayout), p.DaysToQuery)
	} else {
		fmt.Printf("Buscando %s → %s (ida %s, vuelta %s, %d días)\n",
			origin, dest, p.DepartureDate.Format(dateLayout), p.ReturnDate.Format(dateLayout), p.DaysToQuery)
	}

	start := time.Now()
	result, err := sc.FindCheapestFlights(ctx, p)
	elapsed := time.Since(start).Round(time.Second)

	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n\n", err)
		return
	}

	fmt.Printf("  Consultas: %s\n\n", elapsed)

	if len(result.OutboundPerDay) > 0 {
		fmt.Println("  VUELOS DE IDA")
		printResults(result.OutboundPerDay, result.OutboundCheapest, "  ")
	} else {
		fmt.Println("  No se encontraron vuelos de ida")
	}

	if !oneWay {
		if len(result.ReturnPerDay) > 0 {
			fmt.Println("  VUELOS DE VUELTA")
			printResults(result.ReturnPerDay, result.ReturnCheapest, "  ")
		} else {
			fmt.Println("  No se encontraron vuelos de vuelta")
		}
	}
}

func printResults(perDay []client.CheapestFlight, cheapest *client.CheapestFlight, indent string) {
	for _, cf := range perDay {
		fmt.Printf("%s%s: %s-%s, %s, %s, %d escalas, %d millas, USD %.2f tasas\n",
			indent,
			cf.Flight.Departure.Date.Format(dateLayout),
			cf.Flight.Departure.Airport.Code,
			cf.Flight.Arrival.Airport.Code,
			cf.Flight.Cabin,
			cf.Flight.Airline.Name,
			cf.Flight.Stops,
			cf.Fare.Miles,
			float64(cf.Fare.AirlineFareAmt),
		)
	}

	fmt.Println()
	if cheapest != nil {
		fmt.Printf("%s★ Más barato: %s, %s-%s, %s, %s, %d escalas, %d millas, USD %.2f tasas\n",
			indent,
			cheapest.Flight.Departure.Date.Format(dateLayout),
			cheapest.Flight.Departure.Airport.Code,
			cheapest.Flight.Arrival.Airport.Code,
			cheapest.Flight.Cabin,
			cheapest.Flight.Airline.Name,
			cheapest.Flight.Stops,
			cheapest.Fare.Miles,
			float64(cheapest.Fare.AirlineFareAmt),
		)
	}
	fmt.Println()
}

func parseArgs(args []string, oneWay bool) (client.RoundTripParams, error) {
	var p client.RoundTripParams
	p.OneWay = oneWay

	if oneWay {
		// args: [departureDate, days]
		dep, err := time.Parse(dateLayout, args[0])
		if err != nil {
			return p, fmt.Errorf("la fecha de salida %s no es válida: %v", args[0], err)
		}
		days, err := strconv.Atoi(args[1])
		if err != nil {
			return p, fmt.Errorf("la cantidad de días %s no es válida: %v", args[1], err)
		}
		if days < 1 || days > 31 {
			return p, fmt.Errorf("la cantidad de días debe ser entre 1 y 31")
		}
		p.DepartureDate = dep
		p.DaysToQuery = days
	} else {
		// args: [departureDate, returnDate, days]
		dep, err := time.Parse(dateLayout, args[0])
		if err != nil {
			return p, fmt.Errorf("la fecha de salida %s no es válida: %v", args[0], err)
		}
		ret, err := time.Parse(dateLayout, args[1])
		if err != nil {
			return p, fmt.Errorf("la fecha de regreso %s no es válida: %v", args[1], err)
		}
		if ret.Before(dep) {
			return p, fmt.Errorf("la fecha de regreso debe ser posterior a la de salida")
		}
		days, err := strconv.Atoi(args[2])
		if err != nil {
			return p, fmt.Errorf("la cantidad de días %s no es válida: %v", args[2], err)
		}
		if days < 1 || days > 31 {
			return p, fmt.Errorf("la cantidad de días debe ser entre 1 y 31")
		}
		p.DepartureDate = dep
		p.ReturnDate = ret
		p.DaysToQuery = days
	}

	return p, nil
}
