# Smiles
Consultar la API de Smiles buscando los vuelos más baratos con millas

## Requisitos

- Go 1.22+
- Variable de entorno `SMILES_API_KEY` con tu API key de Smiles

## CLI

### Descargar
Descargar del último release el binario `smiles` compatible con tu sistema operativo y arquitectura.

### Uso
```bash
export SMILES_API_KEY="tu-api-key"
smiles <Origen> <Destino> <FechaSalida> <FechaVuelta> <DíasAConsultar>
```

Ejemplo:
```bash
smiles EZE PUJ 2024-01-10 2024-01-20 5
```

### Compilar desde el código fuente
```bash
go build -o smiles ./cmd/cli
```

## MCP Server

El servidor MCP expone las capacidades de búsqueda de vuelos como herramientas para LLMs (Claude, etc).

### Compilar
```bash
go build -o smiles-mcp ./cmd/mcp
```

### Configuración en Claude Code / Claude Desktop

```json
{
  "mcpServers": {
    "smiles": {
      "command": "/ruta/a/smiles-mcp",
      "env": {
        "SMILES_API_KEY": "tu-api-key"
      }
    }
  }
}
```

### Tools disponibles

| Tool | Descripción |
|------|-------------|
| `search_flights` | Busca vuelos en una fecha entre dos aeropuertos con todas las tarifas disponibles |
| `find_cheapest_flights` | Busca el vuelo más barato ida y vuelta en un rango de fechas |
| `get_flight_taxes` | Obtiene el desglose de tasas e impuestos para un vuelo específico |

## Estructura del proyecto

```
smiles/
  cmd/cli/main.go       # CLI
  cmd/mcp/main.go       # MCP server
  client/client.go      # Cliente HTTP compartido
  model/model.go        # Modelos de datos
  server/server.go      # Definiciones y handlers MCP
```
