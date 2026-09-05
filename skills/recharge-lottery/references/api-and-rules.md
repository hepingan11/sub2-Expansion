# API And Rules

## Eligibility

The backend reads Sub2API payment orders and includes only orders with:

- `status=COMPLETED`
- `order_type=balance`
- `paid_at >= periodStart`
- `paid_at < periodEnd`

It aggregates `amount` by user. A user is eligible when the total is greater than or equal to `minRecharge`. The default window is one calendar month before the current instant through the current instant.

## Expansion API

All routes require a Sub2 Expansion admin bearer token.

### Preview

```http
GET /api/admin/lotteries/eligibility?periodStart=<RFC3339>&periodEnd=<RFC3339>&minRecharge=5
```

The response includes `eligibleCount`, up to 200 candidates, and `truncated`. Candidate objects contain `userName`, `userEmail`, `rechargeAmount`, and `orderCount`. Keep `userId` internal unless explicitly requested.

### Draw

```http
POST /api/admin/lotteries/draw
Content-Type: application/json

{
  "requestId": "unique-client-generated-value",
  "name": "八月充值抽奖",
  "periodStart": "2026-08-01T00:00:00+08:00",
  "periodEnd": "2026-09-01T00:00:00+08:00",
  "minRecharge": 5,
  "winnerCount": 3,
  "prizeAmount": 10
}
```

The backend reloads and validates candidates, uses cryptographically secure randomness without replacement, stores a complete candidate snapshot, and credits each winner through the Sub2API admin balance endpoint.

### History And Retry

```http
GET /api/admin/lotteries
POST /api/admin/lotteries/:id/retry-awards
```

History returns the latest 50 draws. Retry processes only pending, failed, or stale processing awards and does not redraw winners.

## Award Status

- `AWARDED`: Sub2API confirmed the balance credit.
- `PENDING`: selected but not yet sent.
- `PROCESSING`: delivery is in progress. A record older than 10 minutes is recoverable by retry.
- `FAILED`: delivery failed; inspect `awardError` before retrying.

Sub2API rejects an admin balance addition when the resulting balance would still be negative. For example, a user at `-0.11215174` needs a prize of at least `0.12`; a `0.01` credit is rejected. The expansion backend records a readable minimum-amount message for this case. Do not silently forgive the deficit or change the prize because that changes the authorized payout.

## Authentication

Login endpoint:

```http
POST /api/admin/login
Content-Type: application/json

{"username":"...","password":"..."}
```

Use the returned `token` as `Authorization: Bearer <token>`. Never log the password or token.
