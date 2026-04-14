package main

import (
	"fmt"
	"os"
	"smiles/client"
	mcpserver "smiles/server"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	apiKey := os.Getenv("SMILES_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: SMILES_API_KEY environment variable is required")
		os.Exit(1)
	}

	bearerToken := os.Getenv("SMILES_BEARER_TOKEN")
	sc := client.New(apiKey, bearerToken)
	s := mcpserver.NewSmilesServer(sc)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
