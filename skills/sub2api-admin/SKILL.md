---
name: sub2api-admin
description: Manage Sub2API admin and Sub2 Expansion check-in APIs through a bundled CLI and reference guide. Use when the user mentions Sub2API admin API, Admin API Key, account management, redeem codes, recharge codes, invitation codes, inviting new users, invitation rewards or records, groups, proxies, error passthrough rules, TLS fingerprint profiles, imports, exports, batch account updates, CRS sync, social account binding, direct check-in, social check-in, or asks to inspect or change Sub2API/Sub2 Expansion backend state.
---

# Sub2API Admin

Use the bundled CLI instead of ad hoc `curl`. Run examples from this skill directory.

```bash
export SUB2API_BASE_URL='https://your-sub2api-host'
export SUB2_EXPANSION_BASE_URL='https://your-sub2-expansion-host'
export SUB2API_ADMIN_API_KEY='admin-...'
node scripts/sub2api-admin.js accounts list --page-size 20
```

Use separate expansion-admin credentials for invitation records and statistics:

```bash
export SUB2_EXPANSION_ADMIN_USERNAME='admin'
export SUB2_EXPANSION_ADMIN_PASSWORD='...'
```

If no Admin API Key is available, the CLI can log in with an admin account for the current process:

```bash
export SUB2API_ADMIN_EMAIL='admin@example.com'
export SUB2API_ADMIN_PASSWORD='...'
node scripts/sub2api-admin.js redeem-codes list --page-size 20
```

For full commands and payload examples, read [references/admin-cli.md](references/admin-cli.md).

## Workflow

1. Confirm `SUB2API_BASE_URL` and credentials are available from environment variables. Never ask the user to paste secrets into chat if they can set env vars locally.
2. Prefer read-only commands first: `accounts list`, `accounts get <id>`, `groups all`, `proxies all`, `redeem-codes list`, or `api GET ...`.
3. Before destructive or bulk writes, print the target names and IDs, plus the exact command or payload.
4. Use `--idempotency-key` for payment, recharge, or redeem-code create/redeem workflows.
5. After a write, run a follow-up read command to verify the final state.

## Invitations

For a new-user invitation, have the inviter provide their own invitation code. Request the binding link with `checkins social --invite-code`; if the response contains `SOCIAL_ACCOUNT_NOT_BOUND`, give the returned `bindingUrl` only to that social-platform user. The user must sign in through that link to bind the platform account and invitation code. Do not attempt to bind an invitation on their behalf or disclose the link to another user.

The expansion backend validates the invite code, account creation-time threshold, self-invites, existing bindings, and reward delivery. Treat `REWARDED` as the only successful reward state. Read [references/admin-cli.md](references/admin-cli.md) for commands, statuses, and endpoint details.

## Common Commands

```bash
node scripts/sub2api-admin.js accounts list --page-size 20
node scripts/sub2api-admin.js accounts get 40
node scripts/sub2api-admin.js accounts usage 40
node scripts/sub2api-admin.js accounts set-schedulable 40 true
node scripts/sub2api-admin.js accounts bulk-update --ids 40,39 --json '{"concurrency":10}'
node scripts/sub2api-admin.js groups all
node scripts/sub2api-admin.js proxies all
node scripts/sub2api-admin.js redeem-codes list --page-size 20
node scripts/sub2api-admin.js redeem-codes generate --json '{"count":1,"type":"balance","value":10}' --idempotency-key redeem-123
node scripts/sub2api-admin.js redeem-codes create-and-redeem --json '{"code":"order_123","type":"balance","value":10,"user_id":123}' --idempotency-key order-123
node scripts/sub2api-admin.js checkins social --platform telegram --user-id 12345
node scripts/sub2api-admin.js checkins social --platform qq --user-id 12345 --invite-code 6A2W7FQC
node scripts/sub2api-admin.js invitations list --status REWARDED --page-size 20
node scripts/sub2api-admin.js invitations stats
node scripts/sub2api-admin.js error-rules list
node scripts/sub2api-admin.js tls-profiles list
node scripts/sub2api-admin.js api GET /admin/settings/admin-api-key
```

## Safety Notes

- Authentication uses `x-api-key` from `SUB2API_ADMIN_API_KEY` first. If absent, the CLI logs in with `SUB2API_ADMIN_EMAIL` and `SUB2API_ADMIN_PASSWORD` and sends a process-local bearer token.
- If the API returns `INVALID_ADMIN_KEY`, ask the user to regenerate the Admin API Key in Sub2API settings.
- `accounts export` may include credentials and tokens. Prefer `--file` and avoid printing exports in chat.
- Delete, batch-delete, bulk-update, credential import, CRS sync, and account refresh can affect production state. Verify target IDs first.
- For new backend APIs not yet wrapped by the CLI, use `api <METHOD> <admin-path>` after a read-only check.
