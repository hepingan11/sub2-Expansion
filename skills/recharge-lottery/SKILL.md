---
name: recharge-lottery
description: Preview eligible Sub2 Expansion users by completed balance recharge amount, run confirmed random lottery draws with a configurable winner count and per-winner balance prize, inspect draw history, and retry failed awards. Use for recharge-user lotteries, monthly recharge draws, winner selection, or lottery award delivery; do not use for unrelated Sub2API administration.
---

# Recharge Lottery

Use `scripts/recharge-lottery.js` for all lottery operations. It calls the Sub2 Expansion backend, which revalidates eligibility and performs the cryptographically random draw. Never select winners locally, edit the database, or modify the `sub2api` project.

Set credentials in the environment. Do not ask the user to paste secrets into chat.

```bash
export SUB2_EXPANSION_BASE_URL='https://your-expansion-host'
export SUB2_EXPANSION_ADMIN_USERNAME='admin'
export SUB2_EXPANSION_ADMIN_PASSWORD='...'
```

`SUB2_EXPANSION_ADMIN_TOKEN` can replace the username and password when a valid token is already available.

## Workflow

1. Preview eligibility first unless the user explicitly asks to skip preview. Defaults are the previous calendar month through now and a minimum completed balance recharge of 5.
2. Report the time range, minimum recharge, eligible-user count, and candidates by name and email. Do not display user IDs unless the user explicitly requests them.
3. Before a draw, state the winner count, prize per winner, total maximum payout, and that balance awards are real external writes. Obtain explicit confirmation immediately before running the command.
4. Execute `draw` with `--confirm`. The backend reloads payment orders, so the preview is informative rather than authoritative.
5. Report winners by name and email, their award status, and the recorded draw ID. Verify with `list` after the write.
6. Retry only an existing failed or stalled award, and obtain confirmation before `retry --confirm`. Stop after one retry attempt unless the user authorizes another.

Common commands:

```bash
node scripts/recharge-lottery.js preview
node scripts/recharge-lottery.js preview --start 2026-08-01T00:00:00+08:00 --end 2026-09-01T00:00:00+08:00 --min-recharge 5
node scripts/recharge-lottery.js draw --name '八月充值抽奖' --winners 3 --prize 10 --confirm
node scripts/recharge-lottery.js list
node scripts/recharge-lottery.js retry --id 12 --confirm
```

Read [references/api-and-rules.md](references/api-and-rules.md) when diagnosing eligibility, award failures, time boundaries, or API errors.

## Safety

- `draw` and `retry` are mutating operations. Never add `--confirm` based only on an inferred intent.
- Winner count must be `1..1000`; minimum recharge and prize must be positive; prize cannot exceed `1000000` per winner. The backend enforces these again.
- A draw request uses a unique request ID to prevent accidental duplicate execution.
- Award retries reuse the backend's fixed idempotency key, so a successful balance credit is not duplicated.
- Treat only `AWARDED` as successfully delivered. Clearly report `PENDING`, `PROCESSING`, and `FAILED`.
- Do not compensate for a failed award by creating another draw.
