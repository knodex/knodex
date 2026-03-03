/**
 * Playwright Global Setup
 * Runs once before all tests start
 */

import { spawn, ChildProcess } from 'child_process'
import { fullCleanup } from './fixture/cleanup'

// Store the OIDC server process globally for teardown
declare global {
  var __oidcServerProcess: ChildProcess | undefined
}

async function startMockOidcServer(): Promise<void> {
  const MOCK_OIDC_PORT = process.env.MOCK_OIDC_PORT || '4444'
  const mockOidcUrl = `http://localhost:${MOCK_OIDC_PORT}`

  // Check if server is already running
  try {
    const response = await fetch(`${mockOidcUrl}/health`)
    if (response.ok) {
      console.log(`Mock OIDC server already running at ${mockOidcUrl}`)
      return
    }
  } catch {
    // Server not running, start it
  }

  console.log(`Starting Mock OIDC server on port ${MOCK_OIDC_PORT}...`)

  const serverProcess = spawn('node', ['scripts/mock-oidc-server.js'], {
    cwd: process.cwd().replace('/web', ''),
    env: {
      ...process.env,
      PORT: MOCK_OIDC_PORT,
      JWT_SECRET: 'e2e-test-secret-key-for-oidc-testing'
    },
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: false
  })

  global.__oidcServerProcess = serverProcess

  // Wait for server to be ready
  const maxWaitTime = 10000
  const startTime = Date.now()

  while (Date.now() - startTime < maxWaitTime) {
    try {
      const response = await fetch(`${mockOidcUrl}/health`)
      if (response.ok) {
        console.log(`Mock OIDC server started successfully at ${mockOidcUrl}`)
        return
      }
    } catch {
      // Server not ready yet
    }
    await new Promise(resolve => setTimeout(resolve, 100))
  }

  console.warn('Mock OIDC server may not have started properly - OIDC tests may fail')
}

export default async function globalSetup() {
  console.log('\n========================================')
  console.log('Starting E2E Test Suite')
  console.log('========================================\n')

  // Start mock OIDC server for OIDC authentication tests
  await startMockOidcServer()

  // Clean up test data from previous runs
  await fullCleanup()

  console.log('\n========================================')
  console.log('Test environment ready')
  console.log('========================================\n')
}
