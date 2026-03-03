/**
 * E2E Test Data Cleanup Utilities
 *
 * Cleans up test organizations to prevent API slowdowns caused by
 * accumulated test data from multiple test runs.
 */

import { exec } from "child_process";
import { promisify } from "util";

const execAsync = promisify(exec);

/**
 * Delete all test organizations from Kubernetes
 * Matches organizations with names starting with common test prefixes
 */
export async function cleanupTestOrganizations(): Promise<void> {
  try {
    console.log("[Cleanup] Removing test organizations...");

    // Get all organizations matching test patterns
    const { stdout } = await execAsync(
      `kubectl get organizations.knodex.io --all-namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\\n"}{end}' | grep -E "^(org-|test-|alpha-|beta-|gamma-|delta-|edit-|members-|namespace-)" || true`
    );

    if (!stdout.trim()) {
      console.log("[Cleanup] No test organizations found");
      return;
    }

    const orgs = stdout.trim().split("\n").filter(Boolean);
    console.log(`[Cleanup] Found ${orgs.length} test organizations to delete`);

    // Delete organizations in batches to avoid overwhelming the API
    const batchSize = 10;
    for (let i = 0; i < orgs.length; i += batchSize) {
      const batch = orgs.slice(i, i + batchSize);
      const deletePromises = batch.map((org) =>
        execAsync(
          `kubectl delete organizations.knodex.io ${org} --ignore-not-found=true 2>&1 || true`
        )
      );
      await Promise.all(deletePromises);
      console.log(
        `[Cleanup] Deleted batch ${Math.floor(i / batchSize) + 1}/${Math.ceil(orgs.length / batchSize)}`
      );
    }

    console.log("[Cleanup] ✓ Test organizations cleaned up successfully");
  } catch (error) {
    console.error("[Cleanup] Failed to clean up test organizations:", error);
    // Don't throw - cleanup failures shouldn't prevent tests from running
  }
}

/**
 * Delete test instances to free up cluster resources
 */
export async function cleanupTestInstances(): Promise<void> {
  try {
    console.log("[Cleanup] Removing test instances...");

    // Delete instances in test namespaces
    const { stdout } = await execAsync(
      `kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\\n"}{end}' | grep -E "^(org-|test-)" || true`
    );

    if (!stdout.trim()) {
      console.log("[Cleanup] No test namespaces found");
      return;
    }

    const namespaces = stdout.trim().split("\n").filter(Boolean);
    console.log(`[Cleanup] Found ${namespaces.length} test namespaces`);

    // Delete all instances in test namespaces
    for (const ns of namespaces) {
      try {
        await execAsync(
          `kubectl delete simpleapp,webapp,microservicesplatform --all -n ${ns} --ignore-not-found=true 2>&1 || true`
        );
      } catch (error) {
        // Ignore errors - namespace might not exist or have no instances
      }
    }

    console.log("[Cleanup] ✓ Test instances cleaned up successfully");
  } catch (error) {
    console.error("[Cleanup] Failed to clean up test instances:", error);
    // Don't throw - cleanup failures shouldn't prevent tests from running
  }
}

/**
 * Clean up test namespaces (empty ones only)
 */
export async function cleanupTestNamespaces(): Promise<void> {
  try {
    console.log("[Cleanup] Removing empty test namespaces...");

    const { stdout } = await execAsync(
      `kubectl get namespaces -o jsonpath='{range .items[*]}{.metadata.name}{"\\n"}{end}' | grep -E "^(org-|test-)" || true`
    );

    if (!stdout.trim()) {
      console.log("[Cleanup] No test namespaces found");
      return;
    }

    const namespaces = stdout.trim().split("\n").filter(Boolean);

    for (const ns of namespaces) {
      try {
        // Check if namespace is empty (no pods)
        const { stdout: pods } = await execAsync(
          `kubectl get pods -n ${ns} --no-headers 2>&1 || true`
        );

        if (!pods.trim()) {
          await execAsync(
            `kubectl delete namespace ${ns} --ignore-not-found=true 2>&1 || true`
          );
          console.log(`[Cleanup] Deleted empty namespace: ${ns}`);
        }
      } catch (error) {
        // Ignore errors
      }
    }

    console.log("[Cleanup] ✓ Empty test namespaces cleaned up");
  } catch (error) {
    console.error("[Cleanup] Failed to clean up test namespaces:", error);
  }
}

/**
 * Full cleanup - organizations, instances, and namespaces
 */
export async function fullCleanup(): Promise<void> {
  console.log("[Cleanup] Starting full test data cleanup...");
  await cleanupTestInstances();
  await cleanupTestOrganizations();
  await cleanupTestNamespaces();
  console.log("[Cleanup] ✓ Full cleanup complete");
}
