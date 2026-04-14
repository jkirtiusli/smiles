# Smiles

Buscador de vuelos baratos con millas Smiles. CLI para búsquedas rápidas desde la terminal y servidor MCP para usar desde Claude u otros LLMs.

## Requisitos

- Go 1.22+
- Variable de entorno `SMILES_API_KEY` (requerida)
- Variable de entorno `SMILES_BEARER_TOKEN` (opcional, mejora autenticación)

Las keys se obtienen inspeccionando las requests de red en [smiles.com.ar](https://www.smiles.com.ar) (DevTools → Network → headers `x-api-key` y `Authorization`).

## CLI

### Descargar

Descargar del [último release](https://github.com/jkirtiusli/smiles/releases) el binario `smiles` compatible con tu sistema operativo y arquitectura.

### Uso

```bash
export SMILES_API_KEY="tu-api-key"
export SMILES_BEARER_TOKEN="tu-bearer-token"
```

**Solo ida** (múltiples destinos, hasta 31 días):
```bash
smiles EZE MAD,BCN,FCO,MXP 2026-06-01 30
```

**Ida y vuelta**:
```bash
smiles EZE PUJ 2026-05-15 2026-05-25 10
```

### Ejemplo de salida

```
Buscando ida EZE → FCO (2026-06-01, 10 días)
  Consultas: 12s

  VUELOS DE IDA
  2026-06-07: EZE-FCO, ECONOMIC, AIR FRANCE, 1 escalas, 297600 millas, USD 315.76 tasas
  2026-06-08: EZE-FCO, ECONOMIC, AIR FRANCE, 1 escalas, 297600 millas, USD 315.76 tasas
  2026-06-01: EZE-FCO, ECONOMIC, AEROLINEAS, 0 escalas, 409700 millas, USD 280.50 tasas

  ★ Más barato: 2026-06-07, EZE-FCO, ECONOMIC, AIR FRANCE, 1 escalas, 297600 millas, USD 315.76 tasas
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
        "SMILES_API_KEY": "tu-api-key",
        "SMILES_BEARER_TOKEN": "tu-bearer-token"
      }
    }
  }
}
```

### Tools disponibles

| Tool | Descripción |
|------|-------------|
| `search_flights` | Busca vuelos en una fecha entre dos aeropuertos con todas las tarifas disponibles |
| `find_cheapest_flights` | Busca el vuelo más barato en un rango de fechas (ida y vuelta o solo ida) |
| `get_flight_taxes` | Obtiene el desglose de tasas e impuestos para un vuelo específico |

## Estructura del proyecto

```
smiles/
  cmd/cli/main.go       # CLI (solo ida, multi-destino, hasta 31 días)
  cmd/mcp/main.go       # MCP server (3 tools para LLMs)
  client/client.go      # Cliente HTTP con bypass de Akamai (tls-client)
  model/model.go        # Modelos de datos de la API de Smiles
  server/server.go      # Definiciones y handlers MCP
  data/response.json    # Fixture para tests
```

## Tests

```bash
go test ./...
```
