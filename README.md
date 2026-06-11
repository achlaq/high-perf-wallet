# high-perf-wallet

A high-performance, concurrency-safe, enterprise-grade digital ledger core built in Go.

This application is designed to handle heavy concurrent transfer requests between digital wallets safely, preventing database deadlocks and double-spending while maintaining data consistency.

---

## Key Features

*   **Pessimistic Locking (`SELECT FOR UPDATE`)**: Prevents race conditions and double-spending by locking the database rows of both accounts involved in a transfer.
*   **Deadlock Prevention**: Automatically sorts account IDs before acquiring row locks, eliminating circular wait conditions (deadlocks) during concurrent bidirectional transfers.
*   **Redis-Powered Idempotency (`X-Idempotency-Key`)**: Avoids duplicate request executions by caching response statuses and payloads in Redis using atomic `SetNX` operations.
*   **Embedded Database Migrations**: Uses Go's native `embed` package to compile SQL migration scripts directly into the application binary, executing database migrations automatically upon server startup.
*   **Graceful Shutdown**: Listens to system signals (SIGINT, SIGTERM) to stop accepting new requests, complete active in-flight transfers, and cleanly close database and cache connections before exit.
*   **Enterprise Clean Architecture**: Structured layout dividing concerns into Domain, Repository, Usecase, and HTTP Delivery layers to maximize testability and maintainability.
*   **Request Tracing Middleware**: Generates and attaches a unique Request ID (`X-Request-ID`) to the response headers for request lifecycle tracing and debugging.

---

## Tech Stack

*   **Language**: Go (v1.26+)
*   **HTTP Router**: Gin Gonic
*   **Database**: PostgreSQL 16
*   **Database Driver**: pgx (v5)
*   **In-Memory Cache**: Redis 7
*   **Infrastructure**: Docker & Docker Compose
*   **Logger**: Uber Zap (Structured Logging)

---

## Getting Started

### Prerequisites
Make sure you have Go and Docker installed on your system.

### 1. Start Infrastructure
Run the database and cache containers in the background:
```bash
docker compose up -d
```

### 2. Run the Application
Execute the Go application. The migration runner will automatically run SQL scripts on startup to initialize and seed the tables:
```bash
go run cmd/api/main.go
```

---

## API & Testing Guide

### 1. Get Wallet Balance
Fetch the current balance and details of a wallet:
```bash
curl -i http://localhost:8080/api/v1/wallets/1
```
*Note: The response will include a unique `X-Request-ID` header.*

### 2. Execute Transfer (Idempotent)
Send funds from Alice (ID: 1) to Bob (ID: 2) using an idempotency key:
```bash
curl -i -X POST http://localhost:8080/api/v1/wallets/transfer \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: transfer-uuid-101" \
  -d '{"from_account_id": 1, "to_account_id": 2, "amount": 5000}'
```
If you replay the request with the same `X-Idempotency-Key`, the API will return the cached response immediately from Redis without executing the transfer again.

### 3. Get Transfer History
Retrieve the transaction history of a specific account:
```bash
curl -i http://localhost:8080/api/v1/wallets/1/transfers
```

### 4. Test Graceful Shutdown
Press `Ctrl + C` in the application console. The logs will display the server completing active connections, closing DB/Redis resources, and exiting safely.
