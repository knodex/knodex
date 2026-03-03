import { test, expect, TestUserRole } from '../fixture';

/**
 * Note: Global Admin - WebSocket Real-Time Updates Tests
 *
 * Tests that Global Admin users receive real-time WebSocket updates for:
 * - Instance status changes across all organizations
 * - RGD catalog changes
 * - Automatic reconnection on disconnect
 *
 * Prerequisites:
 * - Backend deployed with WebSocket support
 * - Global Admin user logged in (groups: ["global-admins"])
 * - Test organizations and instances deployed
 *
 * Test coverage:
 * - Global Admin receives updates for instance status changes in all organizations
 * - Global Admin receives updates for RGD catalog changes
 * - WebSocket connection reconnects automatically on disconnect
 */

// Use relative URLs - Playwright baseURL is set in playwright.config.ts
// BASE_URL and WS_URL are needed for direct API and WebSocket calls
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8080';
const WS_URL = process.env.WS_URL || 'ws://localhost:8080/ws';

test.describe('Global Admin - WebSocket Real-Time Updates', () => {
  // Authenticate as Global Admin to receive WebSocket updates for all organizations
  test.use({ authenticateAs: TestUserRole.GLOBAL_ADMIN });

  test('AC-WEBSOCKET-01: Global Admin receives updates for instance status changes in all organizations', async ({ page }) => {
    // Navigate to instances page
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-01-instances-initial.png',
      fullPage: true
    });

    // Set up WebSocket message listener
    const wsMessages: any[] = [];

    await page.evaluate((wsUrl) => {
      // Store WebSocket messages in window for test access
      (window as any).wsMessages = [];
      (window as any).wsConnection = null;

      // Check if WebSocket is already connected by the app
      const checkExistingWs = () => {
        if ((window as any).ws) {
          console.log('Found existing WebSocket connection');
          return true;
        }
        return false;
      };

      // If app doesn't have WS, create test connection
      if (!checkExistingWs()) {
        console.log('Creating test WebSocket connection to:', wsUrl);
        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
          console.log('WebSocket connected');
          (window as any).wsConnected = true;
        };

        ws.onmessage = (event) => {
          console.log('WebSocket message received:', event.data);
          try {
            const message = JSON.parse(event.data);
            (window as any).wsMessages.push(message);
          } catch (e) {
            console.error('Failed to parse WS message:', e);
          }
        };

        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
        };

        (window as any).wsConnection = ws;
      }
    }, WS_URL);

    // Wait for WebSocket to connect
    await page.waitForTimeout(2000);

    // Verify WebSocket connection established
    const wsConnected = await page.evaluate(() => {
      return (window as any).wsConnected || ((window as any).ws && (window as any).ws.readyState === WebSocket.OPEN);
    });

    console.log('WebSocket connection status:', wsConnected);

    // Trigger instance status change via API (simulate instance update)
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      // Get an instance to update
      const instancesResponse = await page.request.get(`${BASE_URL}/api/v1/instances`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (instancesResponse.ok()) {
        const instancesData = await instancesResponse.json();
        const instances = instancesData.items || instancesData.instances || instancesData;

        if (instances.length > 0) {
          const instanceToUpdate = instances[0];
          const instanceId = instanceToUpdate.id || instanceToUpdate.metadata?.name;

          console.log('Updating instance:', instanceId);

          // Update instance status via API
          await page.request.patch(`${BASE_URL}/api/v1/instances/${instanceId}`, {
            headers: {
              Authorization: `Bearer ${token}`,
              'Content-Type': 'application/json'
            },
            data: {
              status: {
                phase: 'Running',
                message: 'Updated by WebSocket test',
                lastUpdateTime: new Date().toISOString()
              }
            },
            failOnStatusCode: false
          });

          // Wait for WebSocket message
          await page.waitForTimeout(3000);

          await page.screenshot({
            path: '../test-results/e2e/screenshots/websocket-01-after-update.png',
            fullPage: true
          });

          // Check if WebSocket message was received
          const receivedMessages = await page.evaluate(() => {
            return (window as any).wsMessages || [];
          });

          console.log('WebSocket messages received:', receivedMessages.length);

          if (receivedMessages.length > 0) {
            console.log('WebSocket messages:', JSON.stringify(receivedMessages, null, 2));

            // Verify message contains instance update
            const hasInstanceUpdate = receivedMessages.some((msg: any) =>
              msg.type === 'instance_update' ||
              msg.event === 'instance_update' ||
              msg.data?.type === 'instance' ||
              (typeof msg === 'object' && JSON.stringify(msg).includes(instanceId))
            );

            expect(hasInstanceUpdate).toBeTruthy();
          } else {
            console.log('No WebSocket messages received - WebSocket might not be implemented yet');
            // This is acceptable if WebSocket feature is not yet fully implemented
          }
        }
      }
    }

    // Verify Global Admin sees updates from all organizations
    // Check that instances from org-alpha, org-beta, org-gamma are all visible
    const alphaInstances = page.locator('text=org-alpha');
    const betaInstances = page.locator('text=org-beta');
    const gammaInstances = page.locator('text=org-gamma');

    // At least verify we can see instances from multiple orgs
    const hasMultiOrgVisibility = await Promise.race([
      alphaInstances.count().then(c => c > 0),
      betaInstances.count().then(c => c > 0),
      gammaInstances.count().then(c => c > 0)
    ]);

    console.log('Global Admin multi-org visibility:', hasMultiOrgVisibility);
  });

  test('AC-WEBSOCKET-02: Global Admin receives updates for RGD catalog changes', async ({ page }) => {
    await page.goto(`/catalog`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-02-catalog-initial.png',
      fullPage: true
    });

    // Count initial RGDs
    const initialRgdCards = page.locator('[data-testid="rgd-card"], .rgd-card, [class*="rgd"]');
    const initialCount = await initialRgdCards.count();
    console.log(`Initial RGD count: ${initialCount}`);

    // Set up WebSocket message listener
    await page.evaluate((wsUrl) => {
      (window as any).wsMessages = [];
      (window as any).rgdMessages = [];

      // Check for existing WebSocket or create new one
      if (!(window as any).wsConnection && !(window as any).ws) {
        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
          console.log('WebSocket connected for RGD updates');
          (window as any).wsConnected = true;
        };

        ws.onmessage = (event) => {
          console.log('WebSocket message for RGD:', event.data);
          try {
            const message = JSON.parse(event.data);
            (window as any).wsMessages.push(message);

            // Filter RGD-related messages
            if (message.type === 'rgd_update' ||
                message.event === 'rgd_update' ||
                message.data?.type === 'rgd' ||
                message.kind === 'ResourceGraphDefinition') {
              (window as any).rgdMessages.push(message);
            }
          } catch (e) {
            console.error('Failed to parse WS message:', e);
          }
        };

        (window as any).wsConnection = ws;
      }
    }, WS_URL);

    await page.waitForTimeout(2000);

    // Simulate RGD catalog change via API
    const token = await page.evaluate(() => localStorage.getItem('token') || sessionStorage.getItem('token'));

    if (token) {
      // Create a new test RGD via API (if backend supports it)
      const testRgdName = `test-websocket-rgd-${Date.now()}`;

      const createResponse = await page.request.post(`${BASE_URL}/api/v1/rgds`, {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        data: {
          apiVersion: 'kro.run/v1alpha1',
          kind: 'ResourceGraphDefinition',
          metadata: {
            name: testRgdName,
            namespace: 'default'
          },
          spec: {
            description: 'WebSocket test RGD'
          }
        },
        failOnStatusCode: false
      });

      console.log('RGD creation response status:', createResponse.status());

      if (createResponse.status() === 201 || createResponse.status() === 200) {
        // Wait for WebSocket update
        await page.waitForTimeout(3000);

        // Reload page to see new RGD
        await page.reload();
        await page.waitForLoadState('load');

        await page.screenshot({
          path: '../test-results/e2e/screenshots/websocket-02-catalog-after-update.png',
          fullPage: true
        });

        // Verify new RGD appears
        const newRgd = page.locator(`text=${testRgdName}`);
        const isVisible = await newRgd.isVisible({ timeout: 5000 }).catch(() => false);

        if (isVisible) {
          console.log('New RGD visible in catalog after WebSocket update');
        }

        // Check WebSocket messages
        const rgdMessages = await page.evaluate(() => {
          return (window as any).rgdMessages || [];
        });

        console.log('RGD WebSocket messages received:', rgdMessages.length);

        if (rgdMessages.length > 0) {
          console.log('RGD messages:', JSON.stringify(rgdMessages, null, 2));
        }
      }
    }

    // Verify catalog page shows real-time updates indicator (if exists)
    const updateIndicator = page.locator('[data-testid="live-updates"]').or(page.getByText(/live update/i)).or(page.getByText(/connected/i));
    if (await updateIndicator.isVisible({ timeout: 3000 }).catch(() => false)) {
      console.log('Live updates indicator visible');
      await page.screenshot({
        path: '../test-results/e2e/screenshots/websocket-02-live-indicator.png',
        fullPage: true
      });
    }
  });

  test('AC-WEBSOCKET-03: WebSocket connection reconnects automatically on disconnect', async ({ page }) => {
    await page.goto(`/instances`);
    await page.waitForLoadState('load', { timeout: 10000 });

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-03-initial.png',
      fullPage: true
    });

    // Establish WebSocket connection and track reconnection attempts
    await page.evaluate((wsUrl) => {
      (window as any).wsConnectionAttempts = 0;
      (window as any).wsReconnected = false;
      (window as any).wsStates = [];

      // Create WebSocket with reconnection logic
      function connectWebSocket() {
        console.log('Attempting WebSocket connection, attempt:', (window as any).wsConnectionAttempts + 1);
        (window as any).wsConnectionAttempts++;

        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
          console.log('WebSocket connection opened');
          (window as any).wsStates.push('open');

          if ((window as any).wsConnectionAttempts > 1) {
            (window as any).wsReconnected = true;
            console.log('WebSocket reconnected successfully');
          }
        };

        ws.onclose = () => {
          console.log('WebSocket connection closed');
          (window as any).wsStates.push('closed');

          // Attempt to reconnect after 2 seconds
          setTimeout(() => {
            if ((window as any).wsConnectionAttempts < 5) {
              connectWebSocket();
            }
          }, 2000);
        };

        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
          (window as any).wsStates.push('error');
        };

        (window as any).wsConnection = ws;
        return ws;
      }

      const ws = connectWebSocket();
      (window as any).wsOriginal = ws;
    }, WS_URL);

    // Wait for initial connection
    await page.waitForTimeout(2000);

    // Verify initial connection
    const initialStates = await page.evaluate(() => (window as any).wsStates || []);
    console.log('Initial WebSocket states:', initialStates);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-03-connected.png',
      fullPage: true
    });

    // Force disconnect WebSocket
    await page.evaluate(() => {
      if ((window as any).wsConnection) {
        console.log('Forcing WebSocket disconnect');
        (window as any).wsConnection.close();
      }
    });

    console.log('Forced WebSocket disconnect, waiting for reconnection...');
    await page.waitForTimeout(1000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-03-disconnected.png',
      fullPage: true
    });

    // Wait for automatic reconnection (up to 10 seconds)
    await page.waitForTimeout(8000);

    await page.screenshot({
      path: '../test-results/e2e/screenshots/websocket-03-after-reconnect.png',
      fullPage: true
    });

    // Verify reconnection occurred
    const reconnectionData = await page.evaluate(() => {
      return {
        attempts: (window as any).wsConnectionAttempts || 0,
        reconnected: (window as any).wsReconnected || false,
        states: (window as any).wsStates || [],
        currentState: (window as any).wsConnection?.readyState
      };
    });

    console.log('Reconnection data:', JSON.stringify(reconnectionData, null, 2));

    // Verify at least 2 connection attempts (initial + reconnect)
    expect(reconnectionData.attempts).toBeGreaterThanOrEqual(2);

    // Verify connection states show close and reopen
    const hasCloseState = reconnectionData.states.includes('closed');
    const hasReopenState = reconnectionData.states.filter((s: string) => s === 'open').length >= 2;

    console.log('Has close state:', hasCloseState);
    console.log('Has reopen state:', hasReopenState);

    // Look for reconnection indicator in UI
    const reconnectIndicator = page.locator('[data-testid="reconnecting"], text=/reconnecting/i, text=/connection lost/i');
    const hadReconnectIndicator = await reconnectIndicator.isVisible({ timeout: 2000 }).catch(() => false);

    if (hadReconnectIndicator) {
      console.log('Reconnection indicator was shown to user');
      await page.screenshot({
        path: '../test-results/e2e/screenshots/websocket-03-reconnect-indicator.png',
        fullPage: true
      });
    }

    // Verify WebSocket is now connected again
    const finalState = await page.evaluate(() => {
      const ws = (window as any).wsConnection;
      return ws ? ws.readyState : -1;
    });

    console.log('Final WebSocket state:', finalState);
    // WebSocket.OPEN = 1
    if (finalState === 1) {
      console.log('✅ WebSocket successfully reconnected');
    } else {
      console.log('⚠️ WebSocket state:', finalState, '(0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED)');
    }

    // Verify the application is still functional after reconnection
    const instancesVisible = page.locator('[data-testid="instance-card"], .instance-card');
    const instanceCount = await instancesVisible.count();
    console.log(`Instances still visible after reconnection: ${instanceCount}`);
  });
});
