# nordikcsaaapi

Starter API project for Nordik CSAA.

## What is included

- Go HTTP API using the standard library
- Sample endpoint: `GET /api/v1/sample`
- Auth placeholder endpoints: `POST /api/v1/auth/login` and `POST /api/v1/auth/signup`
- Health endpoint: `GET /health`
- Environment template in `.env.example`
- Dockerfile and docker-compose file
- GitHub Actions for CI, Docker build validation, and CodeQL

`GEMINI_KEY` is intentionally not included yet.

## Run locally

```powershell
Copy-Item .env.example .env
go run ./cmd/server
```

The API listens on `http://localhost:8080` by default.

## Example requests

```powershell
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/api/v1/sample

Invoke-RestMethod `
  -Method Post `
  -ContentType 'application/json' `
  -Body '{"email":"demo@nordik.local","password":"password"}' `
  http://localhost:8080/api/v1/auth/login
```

## Environment

Use `.env.example` as the source of truth for required configuration. Keep real secrets in local `.env` files or GitHub Actions secrets.
