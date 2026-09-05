#!/usr/bin/env node

"use strict";

const { randomUUID } = require("crypto");

const BASE_URL = (process.env.SUB2_EXPANSION_BASE_URL || "").replace(/\/+$/, "");
const ADMIN_USERNAME = process.env.SUB2_EXPANSION_ADMIN_USERNAME || "";
const ADMIN_PASSWORD = process.env.SUB2_EXPANSION_ADMIN_PASSWORD || "";
let adminToken = process.env.SUB2_EXPANSION_ADMIN_TOKEN || "";

function usage() {
  console.log(`Usage:
  recharge-lottery.js preview [--start RFC3339] [--end RFC3339] [--min-recharge 5]
  recharge-lottery.js draw --winners N --prize AMOUNT [--name NAME] [--start RFC3339] [--end RFC3339] [--min-recharge 5] --confirm
  recharge-lottery.js list
  recharge-lottery.js retry --id DRAW_ID --confirm
`);
}

function parseArgs(argv) {
  const positional = [];
  const flags = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) {
      positional.push(token);
      continue;
    }
    const key = token.slice(2);
    const next = argv[i + 1];
    if (!next || next.startsWith("--")) {
      flags[key] = true;
    } else {
      flags[key] = next;
      i += 1;
    }
  }
  return { positional, flags };
}

function previousCalendarMonth(now) {
  const result = new Date(now);
  const day = result.getDate();
  result.setDate(1);
  result.setMonth(result.getMonth() - 1);
  const lastDay = new Date(result.getFullYear(), result.getMonth() + 1, 0).getDate();
  result.setDate(Math.min(day, lastDay));
  return result;
}

function parseInstant(value, fallback, label) {
  const instant = value ? new Date(value) : fallback;
  if (Number.isNaN(instant.getTime())) throw new Error(`${label} must be a valid RFC3339 date-time`);
  return instant.toISOString();
}

function positiveNumber(value, fallback, label, maximum) {
  const number = value === undefined ? fallback : Number(value);
  if (!Number.isFinite(number) || number <= 0 || (maximum !== undefined && number > maximum)) {
    throw new Error(`${label} must be greater than 0${maximum === undefined ? "" : ` and no greater than ${maximum}`}`);
  }
  return number;
}

function positiveInteger(value, label, maximum) {
  const number = Number(value);
  if (!Number.isInteger(number) || number <= 0 || number > maximum) {
    throw new Error(`${label} must be an integer from 1 to ${maximum}`);
  }
  return number;
}

function lotteryWindow(flags) {
  const now = new Date();
  const periodStart = parseInstant(flags.start, previousCalendarMonth(now), "--start");
  const periodEnd = parseInstant(flags.end, now, "--end");
  if (new Date(periodEnd) <= new Date(periodStart)) throw new Error("--end must be later than --start");
  return {
    periodStart,
    periodEnd,
    minRecharge: positiveNumber(flags["min-recharge"], 5, "--min-recharge"),
  };
}

async function parseResponse(response, operation) {
  const text = await response.text();
  let data;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { raw: text };
  }
  if (!response.ok) throw new Error(`${operation} failed: ${(data && data.message) || response.statusText}`);
  return data;
}

async function token() {
  if (adminToken) return adminToken;
  if (!BASE_URL) throw new Error("Missing SUB2_EXPANSION_BASE_URL");
  if (!ADMIN_USERNAME || !ADMIN_PASSWORD) {
    throw new Error("Missing SUB2_EXPANSION_ADMIN_TOKEN or SUB2_EXPANSION_ADMIN_USERNAME/SUB2_EXPANSION_ADMIN_PASSWORD");
  }
  const response = await fetch(`${BASE_URL}/api/admin/login`, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify({ username: ADMIN_USERNAME, password: ADMIN_PASSWORD }),
  });
  const data = await parseResponse(response, "POST /api/admin/login");
  if (!data.token) throw new Error("POST /api/admin/login returned no token");
  adminToken = data.token;
  return adminToken;
}

async function request(method, path, body) {
  if (!BASE_URL) throw new Error("Missing SUB2_EXPANSION_BASE_URL");
  const headers = { Accept: "application/json", Authorization: `Bearer ${await token()}` };
  const options = { method, headers };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  const response = await fetch(`${BASE_URL}${path}`, options);
  return parseResponse(response, `${method} ${path}`);
}

function candidateView(candidate) {
  return {
    name: candidate.userName || "未设置名称",
    email: candidate.userEmail || "未设置邮箱",
    rechargeAmount: candidate.rechargeAmount,
    orderCount: candidate.orderCount,
  };
}

function winnerView(winner) {
  return {
    ...candidateView(winner),
    prizeAmount: winner.prizeAmount,
    awardStatus: winner.awardStatus,
    ...(winner.awardError ? { awardError: winner.awardError } : {}),
    ...(winner.awardedAt ? { awardedAt: winner.awardedAt } : {}),
  };
}

function drawView(draw) {
  return {
    drawId: draw.id,
    name: draw.name,
    periodStart: draw.periodStart,
    periodEnd: draw.periodEnd,
    minRecharge: draw.minRecharge,
    eligibleCount: draw.eligibleCount,
    winnerCount: draw.winnerCount,
    prizePerWinner: draw.prizeAmount,
    maximumPayout: Number(draw.prizeAmount) * Number(draw.winnerCount),
    winners: (draw.winners || []).map(winnerView),
    createdAt: draw.createdAt,
  };
}

function print(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

async function preview(flags) {
  const window = lotteryWindow(flags);
  const query = new URLSearchParams({
    periodStart: window.periodStart,
    periodEnd: window.periodEnd,
    minRecharge: String(window.minRecharge),
  });
  const result = await request("GET", `/api/admin/lotteries/eligibility?${query}`);
  print({
    periodStart: result.periodStart,
    periodEnd: result.periodEnd,
    minRecharge: result.minRecharge,
    eligibleCount: result.eligibleCount,
    candidates: (result.candidates || []).map(candidateView),
    truncated: Boolean(result.truncated),
  });
}

async function draw(flags) {
  if (!flags.confirm) throw new Error("draw credits real balances; rerun with --confirm after explicit user confirmation");
  if (flags.winners === undefined || flags.prize === undefined) throw new Error("draw requires --winners and --prize");
  const payload = {
    requestId: randomUUID(),
    name: String(flags.name || "充值用户抽奖").trim(),
    ...lotteryWindow(flags),
    winnerCount: positiveInteger(flags.winners, "--winners", 1000),
    prizeAmount: positiveNumber(flags.prize, undefined, "--prize", 1000000),
  };
  if (!payload.name) throw new Error("--name must not be blank");
  print(drawView(await request("POST", "/api/admin/lotteries/draw", payload)));
}

async function listDraws() {
  const draws = await request("GET", "/api/admin/lotteries");
  print((draws || []).map(drawView));
}

async function retry(flags) {
  if (!flags.confirm) throw new Error("retry may credit real balances; rerun with --confirm after explicit user confirmation");
  const id = positiveInteger(flags.id, "--id", Number.MAX_SAFE_INTEGER);
  print(drawView(await request("POST", `/api/admin/lotteries/${id}/retry-awards`)));
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const command = args.positional[0];
  if (!command || command === "help" || args.flags.help) return usage();
  if (command === "preview") return preview(args.flags);
  if (command === "draw") return draw(args.flags);
  if (command === "list") return listDraws();
  if (command === "retry") return retry(args.flags);
  throw new Error(`Unknown command: ${command}`);
}

main().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
