# Simple Wallet Service

A minimal wallet service implemented in **Go** that supports wallet
creation, top-ups, payments, transfers, and suspension. The system
ensures **transactional integrity, concurrency safety, and ledger-based
accounting**.

## Tech Stack

-   Go (`net/http`)
-   PostgreSQL
-   GORM
-   `shopspring/decimal` for precise monetary arithmetic

------------------------------------------------------------------------

## Running the Service

### Start PostgreSQL

``` bash
docker run -d -p 5432:5432 -e POSTGRES_USER=admin -e POSTGRES_PASSWORD=admin -e POSTGRES_DB=wallet postgres:1
```

### Run the Server

``` bash
go run ./cmd/server
```

Server runs at:

http://localhost:8080

------------------------------------------------------------------------

## API Endpoints

Financial operations require header:

X-Idempotency-Key

### Create Wallet

`POST /wallets`

Request

``` json
{
    "owner_id": "user1",
    "currency": "USD"
}
```

Response

``` json
{
    "id": "537c0b9a-663e-43fe-9ffd-61a9f7315da0",
    "owner_id": "user1",
    "currency": "USD",
    "balance": "0",
    "status": "ACTIVE",
    "created_at": "2026-03-04T10:04:44.471486+07:00",
    "updated_at": "2026-03-04T10:04:44.471486+07:00"
}
```

------------------------------------------------------------------------

### Top Up Wallet

`POST /wallets/{id}/topup`

Headers

X-Idempotency-Key: topup-1

Request

``` json
{
    "owner_id": "user2",
    "currency": "USD",
    "amount": "100.00"
}
```

Response

``` json
{
    "id": "537c0b9a-663e-43fe-9ffd-61a9f7315da0",
    "owner_id": "user2",
    "currency": "USD",
    "balance": "899.98",
    "status": "ACTIVE",
    "created_at": "2026-03-04T10:04:44.471486+07:00",
    "updated_at": "2026-03-04T11:55:10.210817+07:00"
}
```

------------------------------------------------------------------------

### Payment

`POST /wallets/{id}/pay`

Headers

X-Idempotency-Key: payment-1

Request

``` json
{
    "owner_id": "user2",
    "currency": "USD",
    "amount": "150.005"
}
```

Response

``` json
{
    "id": "537c0b9a-663e-43fe-9ffd-61a9f7315da0",
    "owner_id": "user2",
    "currency": "USD",
    "balance": "749.97",
    "status": "ACTIVE",
    "created_at": "2026-03-04T10:04:44.471486+07:00",
    "updated_at": "2026-03-04T12:10:03.2719+07:00"
}
}
```

------------------------------------------------------------------------

### Transfer

`POST /wallets/transfer`

Headers

X-Idempotency-Key: transfer-1

Request

``` json
{
    "sender_id": "user2",
    "receiver_id": "user1",
    "sender_currency": "USD",
    "receiver_currency": "USD",
    "amount": "100.00"
}
```

Response

``` json
[
    {
        "id": "537c0b9a-663e-43fe-9ffd-61a9f7315da0",
        "owner_id": "user2",
        "currency": "USD",
        "balance": "649.97",
        "status": "ACTIVE",
        "created_at": "2026-03-04T10:04:44.471486+07:00",
        "updated_at": "2026-03-04T12:10:47.256327+07:00"
    },
    {
        "id": "d0ec79c4-a824-4ab7-99b7-765a918a991b",
        "owner_id": "user1",
        "currency": "USD",
        "balance": "200",
        "status": "ACTIVE",
        "created_at": "2026-03-04T10:04:34.554544+07:00",
        "updated_at": "2026-03-04T12:10:47.25751+07:00"
    }
]
```

------------------------------------------------------------------------

### Suspend Wallet

`POST /wallets/{id}/suspend`

Response

``` json
{
    "id": "b8e16176-626a-4ca7-ba1b-4c3f3c4f048c",
    "owner_id": "user3",
    "currency": "USD",
    "balance": "0",
    "status": "SUSPENDED",
    "created_at": "2026-03-04T12:11:24.629357+07:00",
    "updated_at": "2026-03-04T12:11:40.046072+07:00"
}
```

------------------------------------------------------------------------

### Get Wallet Status

`GET /wallets/{id}`

Response

``` json
{
    "id": "b8e16176-626a-4ca7-ba1b-4c3f3c4f048c",
    "owner_id": "user3",
    "currency": "USD",
    "balance": "0",
    "status": "SUSPENDED",
    "created_at": "2026-03-04T12:11:24.629357+07:00",
    "updated_at": "2026-03-04T12:11:40.046072+07:00"
}
```

------------------------------------------------------------------------

## Design Overview

### Monetary Handling

All amounts use **decimal arithmetic** (`shopspring/decimal`) to avoid
floating-point precision issues.

Amounts are normalized to **2 decimal places** and values below **0.01**
are rejected.

### Ledger Accounting

All balance changes are recorded in an **append-only ledger**.

Invariant:

wallet.balance == SUM(ledger_entries.amount)

Wallet balance acts as a cached value updated transactionally with
ledger entries.

### Concurrency Safety

All financial operations run inside **database transactions**.

Balance updates use:

SELECT ... FOR UPDATE

This prevents race conditions and ensures balances cannot go negative.

### Idempotency

Financial operations require an **idempotency key**.

Duplicate requests are prevented using the `idempotency_keys` table.

------------------------------------------------------------------------

## Assumptions

-   A user may have **multiple wallets but only one per currency**
-   **Currency conversion is not supported**
-   **Ledger entries are immutable**
-   **Self-transfers are rejected**
-   **Suspended wallets cannot perform financial operations**
-   PostgreSQL is used for **transactional guarantees and row-level
locking**

------------------------------------------------------------------------

## Testing

Unit tests cover:

-   wallet creation and uniqueness
-   top-up, payment, and transfer flows
-   idempotency protection
-   suspended wallet restrictions
-   large balance operations
-   concurrent transfers
-   ledger invariants
-   out-of-order requests

Run tests:

```bash
CREATE DATABASE wallet_test;
```

``` bash
go test -v ./internal/service
```
