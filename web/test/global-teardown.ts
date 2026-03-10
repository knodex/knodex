// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Playwright Global Teardown
 * Runs once after all tests complete
 */

import { fullCleanup } from './fixture/cleanup'

// Reference to global OIDC server process
declare global {
  var __oidcServerProcess: import('child_process').ChildProcess | undefined
}

function stopMockOidcServer(): void {
  if (global.__oidcServerProcess) {
    console.log('Stopping Mock OIDC server...')
    global.__oidcServerProcess.kill('SIGTERM')
    global.__oidcServerProcess = undefined
    console.log('Mock OIDC server stopped')
  }
}

export default async function globalTeardown() {
  console.log('\n========================================')
  console.log('E2E Test Suite Complete')
  console.log('========================================\n')

  // Stop mock OIDC server
  stopMockOidcServer()

  // Clean up test data after all tests
  await fullCleanup()

  console.log('\n========================================')
  console.log('Cleanup complete')
  console.log('========================================\n')
}
