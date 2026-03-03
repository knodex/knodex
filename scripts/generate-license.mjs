#!/usr/bin/env node
//
// Generate signed JWT license tokens for Knodex Enterprise.
//
// Usage:
//   node scripts/generate-license.mjs \
//     --key server/ee/license/testdata/test_key.pem \
//     --customer acme-corp \
//     --license-id lic_acme_001 \
//     --features compliance,views \
//     --max-users 50 \
//     --duration 365d
//
// Output file:
//   node scripts/generate-license.mjs --key ... --out license.jwt
//
// No external dependencies - uses Node.js built-in crypto.

import { readFileSync, writeFileSync } from "fs";
import { createSign } from "crypto";
import { parseArgs } from "util";

const ISSUER = "license.knodex.io";

// --- CLI parsing ---

const { values } = parseArgs({
  options: {
    key: { type: "string", short: "k" },
    customer: { type: "string", short: "c" },
    "license-id": { type: "string", short: "l" },
    features: { type: "string", short: "f", default: "compliance,views" },
    "max-users": { type: "string", short: "u", default: "50" },
    edition: { type: "string", short: "e", default: "enterprise" },
    duration: { type: "string", short: "d", default: "365d" },
    out: { type: "string", short: "o" },
    help: { type: "boolean", short: "h" },
  },
  strict: true,
});

if (values.help || !values.key || !values.customer || !values["license-id"]) {
  console.error(`Usage: node scripts/generate-license.mjs [options]

Generates a signed JWT license token for Knodex Enterprise.

Required:
  -k, --key <path>         ECDSA P-256 private key PEM file
  -c, --customer <name>    Customer name (JWT 'sub' claim)
  -l, --license-id <id>    Unique license ID (e.g. lic_acme_001)

Optional:
  -f, --features <list>    Comma-separated features (default: compliance,views)
  -u, --max-users <n>      Max active users (default: 50)
  -e, --edition <name>     License edition (default: enterprise)
  -d, --duration <dur>     License duration: 30d, 365d, 24h (default: 365d)
  -o, --out <path>         Write JWT to file instead of stdout
  -h, --help               Show this help

Example:
  node scripts/generate-license.mjs \\
    --key server/ee/license/testdata/test_key.pem \\
    --customer acme-corp \\
    --license-id lic_acme_001`);
  process.exit(values.help ? 0 : 1);
}

// --- Duration parsing ---

function parseDuration(s) {
  const match = s.match(/^(\d+)([dh])$/);
  if (!match) {
    console.error(`Error: invalid duration "${s}" (use e.g. 30d, 365d, 24h)`);
    process.exit(1);
  }
  const num = parseInt(match[1], 10);
  const unit = match[2];
  if (unit === "d") return num * 86400;
  if (unit === "h") return num * 3600;
}

// --- JWT signing (ES256 with Node.js crypto) ---

function base64url(buf) {
  return Buffer.from(buf)
    .toString("base64")
    .replace(/=/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

function signJWT(payload, privateKeyPem) {
  const header = { alg: "ES256", typ: "JWT" };
  const encodedHeader = base64url(JSON.stringify(header));
  const encodedPayload = base64url(JSON.stringify(payload));
  const signingInput = `${encodedHeader}.${encodedPayload}`;

  // Sign with ECDSA P-256 / SHA-256
  const sign = createSign("SHA256");
  sign.update(signingInput);
  const derSig = sign.sign({ key: privateKeyPem, dsaEncoding: "ieee-p1363" });

  return `${signingInput}.${base64url(derSig)}`;
}

// --- Main ---

const keyPem = readFileSync(values.key, "utf8");
const now = Math.floor(Date.now() / 1000);
const durationSec = parseDuration(values.duration);
const features = values.features
  .split(",")
  .map((f) => f.trim())
  .filter(Boolean);
const maxUsers = parseInt(values["max-users"], 10);

if (features.length === 0) {
  console.error("Error: at least one feature is required");
  process.exit(1);
}

const payload = {
  iss: ISSUER,
  sub: values.customer,
  iat: now,
  exp: now + durationSec,
  features,
  maxUsers,
  licenseId: values["license-id"],
  edition: values.edition,
};

const token = signJWT(payload, keyPem);

if (values.out) {
  writeFileSync(values.out, token, { mode: 0o600 });
  console.error(`License written to ${values.out}`);
} else {
  process.stdout.write(token);
}

// Summary to stderr
const expDate = new Date((now + durationSec) * 1000);
console.error(`
License Summary:
  Customer:   ${values.customer}
  License ID: ${values["license-id"]}
  Edition:    ${values.edition}
  Features:   ${features.join(", ")}
  Max Users:  ${maxUsers}
  Issued:     ${new Date(now * 1000).toISOString()}
  Expires:    ${expDate.toISOString()}`);
