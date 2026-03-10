#!/usr/bin/env node

/**
 * Mock OIDC Server for E2E Testing
 *
 * Simple OIDC server for testing OIDC authentication flow and group mapping.
 * Provides OpenID configuration and token endpoints with configurable group claims.
 *
 * Usage:
 *   node scripts/mock-oidc-server.js
 *
 * Environment Variables:
 *   PORT - Server port (default: 4444)
 *   JWT_SECRET - Secret for signing JWT tokens (default: test-secret-key)
 */

const express = require('express');
const jwt = require('jsonwebtoken');
const cors = require('cors');

const app = express();
const PORT = process.env.PORT || 4444;
const JWT_SECRET = process.env.JWT_SECRET || (() => {
  console.error('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.error('⚠️  SECURITY WARNING: JWT_SECRET not set');
  console.error('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  console.error('');
  console.error('This Mock OIDC Server is for LOCAL E2E TESTING ONLY.');
  console.error('');
  console.error('NEVER expose this server to the internet or shared networks.');
  console.error('NEVER use this server with production credentials.');
  console.error('');
  console.error('Using fallback secret: test-secret-key (INSECURE)');
  console.error('Set JWT_SECRET environment variable for better security.');
  console.error('');
  console.error('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  return 'test-secret-key';
})();
const ISSUER = `http://localhost:${PORT}`;

// Middleware
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Test users with different group mappings
const TEST_USERS = {
  'globaladmin@test.com': {
    sub: 'oidc-global-001',
    email: 'globaladmin@test.com',
    name: 'Global Admin User',
    groups: ['global-admins'],
  },
  'alpha-admin@test.com': {
    sub: 'oidc-alpha-admin-001',
    email: 'alpha-admin@test.com',
    name: 'Alpha Team Admin',
    groups: ['org-alpha-platform-admins'],
  },
  'alpha-dev@test.com': {
    sub: 'oidc-alpha-dev-001',
    email: 'alpha-dev@test.com',
    name: 'Alpha Team Developer',
    groups: ['org-alpha-developers'],
  },
  'alpha-viewer@test.com': {
    sub: 'oidc-alpha-viewer-001',
    email: 'alpha-viewer@test.com',
    name: 'Alpha Team Viewer',
    groups: ['org-alpha-viewers'],
  },
  'multi-group@test.com': {
    sub: 'oidc-multi-001',
    email: 'multi-group@test.com',
    name: 'Multi Group User',
    groups: ['org-alpha-developers', 'org-beta-platform-admins'], // Tests role precedence
  },
  'no-groups@test.com': {
    sub: 'oidc-no-groups-001',
    email: 'no-groups@test.com',
    name: 'No Groups User',
    groups: [],
  },
};

// OpenID Configuration Discovery
app.get('/.well-known/openid-configuration', (req, res) => {
  res.json({
    issuer: ISSUER,
    authorization_endpoint: `${ISSUER}/oauth2/auth`,
    token_endpoint: `${ISSUER}/oauth2/token`,
    userinfo_endpoint: `${ISSUER}/userinfo`,
    jwks_uri: `${ISSUER}/.well-known/jwks.json`,
    response_types_supported: ['code', 'token', 'id_token'],
    subject_types_supported: ['public'],
    id_token_signing_alg_values_supported: ['HS256'],
    scopes_supported: ['openid', 'profile', 'email', 'groups'],
    claims_supported: ['sub', 'email', 'name', 'groups'],
  });
});

// JWKS endpoint (simplified for testing)
app.get('/.well-known/jwks.json', (req, res) => {
  res.json({
    keys: [
      {
        kty: 'oct',
        alg: 'HS256',
        use: 'sig',
        kid: 'test-key-1',
      },
    ],
  });
});

// Authorization endpoint (simplified - just redirects back)
app.get('/oauth2/auth', (req, res) => {
  const { redirect_uri, state, response_type } = req.query;

  // For testing, we'll use the first test user (Global Admin)
  const code = 'test-authorization-code-' + Date.now();

  // Redirect back with authorization code
  const redirectUrl = `${redirect_uri}?code=${code}&state=${state}`;
  res.redirect(redirectUrl);
});

// Token endpoint
app.post('/oauth2/token', (req, res) => {
  const { grant_type, code, redirect_uri } = req.body;

  // Validate required parameters
  if (!grant_type) {
    return res.status(400).json({
      error: 'invalid_request',
      error_description: 'grant_type is required'
    });
  }

  if (grant_type !== 'authorization_code') {
    return res.status(400).json({ error: 'unsupported_grant_type' });
  }

  // Validate authorization code
  if (!code) {
    return res.status(400).json({
      error: 'invalid_request',
      error_description: 'code is required'
    });
  }

  // Prevent excessively long codes (DoS mitigation)
  if (code.length > 200) {
    return res.status(400).json({
      error: 'invalid_grant',
      error_description: 'authorization code too long'
    });
  }

  // Validate code format
  if (!code.startsWith('test-authorization-code-')) {
    return res.status(400).json({
      error: 'invalid_grant',
      error_description: 'invalid authorization code format'
    });
  }

  // Validate redirect_uri (optional but recommended)
  if (redirect_uri && !redirect_uri.startsWith('http://localhost')) {
    console.warn('⚠️  Suspicious redirect_uri:', redirect_uri);
  }

  // Determine user from code suffix with explicit matching
  let user = TEST_USERS['globaladmin@test.com']; // Default

  const codeMatch = code.match(/test-authorization-code-([a-z-]+)-\d+/);
  if (codeMatch) {
    const userHint = codeMatch[1];
    const userEmail = `${userHint}@test.com`;
    if (TEST_USERS[userEmail]) {
      user = TEST_USERS[userEmail];
      console.log(`Token issued for: ${userEmail}`);
    } else {
      console.warn(`Unknown user hint in code: ${userHint}, using globaladmin`);
    }
  }

  const now = Math.floor(Date.now() / 1000);
  const exp = now + 3600; // 1 hour

  // Create ID token with groups claim
  const idToken = jwt.sign(
    {
      iss: ISSUER,
      sub: user.sub,
      aud: 'knodex',
      exp: exp,
      iat: now,
      email: user.email,
      name: user.name,
      groups: user.groups, // Groups claim for mapping
    },
    JWT_SECRET,
    { algorithm: 'HS256' }
  );

  // Create access token (simplified for testing)
  const accessToken = jwt.sign(
    {
      iss: ISSUER,
      sub: user.sub,
      aud: 'knodex',
      exp: exp,
      iat: now,
      scope: 'openid profile email groups',
    },
    JWT_SECRET,
    { algorithm: 'HS256' }
  );

  res.json({
    access_token: accessToken,
    token_type: 'Bearer',
    expires_in: 3600,
    id_token: idToken,
    scope: 'openid profile email groups',
  });
});

// UserInfo endpoint
app.get('/userinfo', (req, res) => {
  const authHeader = req.headers.authorization;
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'unauthorized' });
  }

  const token = authHeader.substring(7);

  try {
    const decoded = jwt.verify(token, JWT_SECRET);

    // Find user by sub
    const user = Object.values(TEST_USERS).find(u => u.sub === decoded.sub);
    if (!user) {
      return res.status(404).json({ error: 'user_not_found' });
    }

    res.json({
      sub: user.sub,
      email: user.email,
      name: user.name,
      groups: user.groups,
    });
  } catch (error) {
    res.status(401).json({ error: 'invalid_token' });
  }
});

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    service: 'mock-oidc-server',
    version: '1.0.0',
    timestamp: new Date().toISOString(),
  });
});

// Test users endpoint (for debugging)
app.get('/test/users', (req, res) => {
  const users = Object.entries(TEST_USERS).map(([email, data]) => ({
    email,
    sub: data.sub,
    name: data.name,
    groups: data.groups,
  }));

  res.json({
    users,
    usage: 'Use the email address in the authorization code to select a specific test user',
    example: 'code=test-authorization-code-alpha-admin-123456',
  });
});

// Start server
app.listen(PORT, () => {
  console.log('========================================');
  console.log('Mock OIDC Server');
  console.log('========================================');
  console.log(`Server running on http://localhost:${PORT}`);
  console.log(`Issuer: ${ISSUER}`);
  console.log('');
  console.log('Endpoints:');
  console.log(`  - Configuration: ${ISSUER}/.well-known/openid-configuration`);
  console.log(`  - Authorization: ${ISSUER}/oauth2/auth`);
  console.log(`  - Token:         ${ISSUER}/oauth2/token`);
  console.log(`  - UserInfo:      ${ISSUER}/userinfo`);
  console.log(`  - Health:        ${ISSUER}/health`);
  console.log(`  - Test Users:    ${ISSUER}/test/users`);
  console.log('');
  console.log('Test Users:');
  Object.entries(TEST_USERS).forEach(([email, data]) => {
    console.log(`  - ${email}: ${data.groups.join(', ') || '(no groups)'}`);
  });
  console.log('');
  console.log('Ready for E2E testing!');
  console.log('========================================');
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully...');
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('\nSIGINT received, shutting down gracefully...');
  process.exit(0);
});
