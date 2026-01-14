# Asset Injector Microservice API Documentation

This directory contains the auto-generated OpenAPI/Swagger documentation for the Asset Injector Microservice.

## Files

- `swagger.json` - OpenAPI specification in JSON format
- `swagger.yaml` - OpenAPI specification in YAML format  
- `docs.go` - Generated Go code for embedding the documentation

## Accessing the Documentation

### Swagger UI

When the service is running, access the interactive Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

### Raw Specification

- JSON: `http://localhost:8080/swagger/doc.json`
- YAML: Available in this directory as `swagger.yaml`

## Regenerating Documentation

```bash
make docs
# Or: swag init -g cmd/server/main.go -o ./docs
```

## API Overview

### Resolution
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/resolve` | Resolve URL pattern to CSS/JS assets |

### Rules Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/rules` | List all rules |
| POST | `/v1/rules` | Create a new rule |
| PUT | `/v1/rules/{id}` | Update an existing rule |
| DELETE | `/v1/rules/{id}` | Delete a rule |
| GET | `/v1/rules/{id}/source` | Get rule origin/attribution info |
| POST | `/v1/rules/export` | Export rules as a pack |

### Pack Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/packs` | List installed packs |
| POST | `/v1/packs/install` | Install a pack from source |
| DELETE | `/v1/packs/{name}` | Uninstall a pack |
| GET | `/v1/packs/available` | List available community packs |
| POST | `/v1/packs/update` | Update installed packs |

### System
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check with component status |
| GET | `/metrics` | Cache and rule statistics |

## Error Responses

All endpoints return standardized errors:

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "Human readable message",
  "details": {}
}
```

### Error Codes
| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_INPUT` | 400 | Malformed request |
| `VALIDATION_FAILED` | 422 | Validation error |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource conflict |
| `RATE_LIMIT` | 429 | Rate limit exceeded |
| `INTERNAL_ERROR` | 500 | Server error |

## Rate Limiting

- Default: 100 requests/second per client
- Burst: 200 requests
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`

## Examples

### Resolve URL Pattern
```bash
curl -X POST http://localhost:8080/v1/resolve \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/page"}'
```

### Create Rule
```bash
curl -X POST http://localhost:8080/v1/rules \
  -H "Content-Type: application/json" \
  -d '{
    "type": "wildcard",
    "pattern": "https://example.com/*",
    "css": ".banner { display: none; }",
    "js": "document.querySelector('.popup').remove();"
  }'
```

### Update Rule
```bash
curl -X PUT http://localhost:8080/v1/rules/{id} \
  -H "Content-Type: application/json" \
  -d '{"css": ".updated { display: none; }"}'
```

### Install Pack
```bash
curl -X POST http://localhost:8080/v1/packs/install \
  -H "Content-Type: application/json" \
  -d '{"source": "cookie-banners@1.0.0"}'
```

### Export Rules
```bash
curl -X POST http://localhost:8080/v1/rules/export \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-pack",
    "version": "1.0.0",
    "description": "My custom rules",
    "author": "me"
  }'
```
