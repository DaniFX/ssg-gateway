# SSG API Gateway

Gateway API scritto in Go, deployato su Google Cloud Platform (Cloud Run). Gestisce autenticazione Firebase e routing verso i microservizi.

## Struttura

```
ssg-gateway/
├── cmd/gateway/main.go       # Entry point
├── internal/
│   ├── config/               # Configurazione
│   ├── handlers/             # HTTP handlers
│   ├── middleware/           # Middleware (auth, logging, etc.)
│   ├── models/               # Modelli dati
│   └── services/             # Servizi esterni (Firebase, etc.)
├── Dockerfile
└── go.mod
```

## Prerequisiti

- Go 1.21+
- Firebase project configurato

## Configurazione

1. Copia `.env.example` a `.env`:

```bash
cp .env.example .env
```

2. Inserisci le credenziali Firebase nel file `.env`:

```
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"
FIREBASE_CLIENT_EMAIL=firebase-adminsdk@your-project.iam.gserviceaccount.com
FIRESTORE_PROJECT_ID=your-project-id
```

## Sviluppo Locale

```bash
go run cmd/gateway/main.go
```

Il server partirà su `http://localhost:8080`

## Build Docker

```bash
docker build -t ssg-gateway .
docker run -p 8080:8080 --env-file .env ssg-gateway
```

## Endpoint

| Metodo | Path | Descrizione | Auth |
|--------|------|-------------|------|
| GET | /health | Health check | No |
| GET | /ready | Readiness probe | No |
| GET | /live | Liveness probe | No |
| GET | /api/v1/public | Endpoint pubblico | No |
| GET | /api/v1/me | Profilo utente | Firebase JWT |
| GET | /api/v1/admin/stats | Statistiche admin | Firebase JWT (role: admin) |

## Autenticazione

Tutti gli endpoint protetti richiedono un JWT token Firebase nell'header:

```
Authorization: Bearer <firebase_id_token>
```

### Flusso

1. Frontend effettua login con Firebase SDK
2. Firebase restituisce ID token
3. Invia richieste con header `Authorization: Bearer <token>`
4. Gateway verifica il token con Firebase
5. Token valido → accesso, invalido → 401

### Ruoli

- `admin` - Accesso completo
- `user` - Utente autenticato
- `guest` - Accesso pubblico

## Variabili d'Ambiente

| Variabile | Default | Descrizione |
|-----------|---------|-------------|
| PORT | 8080 | Porta del server |
| ENVIRONMENT | development | Ambiente di esecuzione |
| FIREBASE_PROJECT_ID | - | ID progetto Firebase |
| FIREBASE_PRIVATE_KEY | - | Chiave privata service account |
| FIREBASE_CLIENT_EMAIL | - | Email service account |
| FIRESTORE_PROJECT_ID | - | ID progetto Firestore |

## Deploy su Cloud Run

```bash
gcloud run deploy ssg-gateway \
  --source . \
  --project YOUR_PROJECT_ID \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars FIREBASE_PROJECT_ID=...,FIREBASE_CLIENT_EMAIL=...,FIREBASE_PRIVATE_KEY=...
```

## Response Format

Success:
```json
{
  "success": true,
  "data": { }
}
```

Error:
```json
{
  "success": false,
  "error": {
    "code": "ERR_CODE",
    "message": "Descrizione errore"
  }
}
```
