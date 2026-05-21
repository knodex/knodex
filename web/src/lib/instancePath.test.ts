// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { buildInstanceRoute, apiGroupOf } from "./instancePath";

describe("buildInstanceRoute", () => {
  it("builds a namespaced route with group, ns, kind, name", () => {
    expect(
      buildInstanceRoute({
        apiVersion: "apps.example.com/v1",
        namespace: "default",
        kind: "WebApp",
        name: "my-app",
      })
    ).toBe("/instances/group/apps.example.com/ns/default/WebApp/my-app");
  });

  it("builds a cluster-scoped route when namespace is absent", () => {
    expect(
      buildInstanceRoute({
        apiVersion: "cert-manager.io/v1",
        kind: "Certificate",
        name: "wildcard",
      })
    ).toBe("/instances/group/cert-manager.io/cluster/Certificate/wildcard");
  });

  it("treats empty-string namespace as cluster-scoped", () => {
    expect(
      buildInstanceRoute({
        apiVersion: "policy.example.com/v1alpha1",
        namespace: "",
        kind: "GlobalPolicy",
        name: "default-policy",
      })
    ).toBe("/instances/group/policy.example.com/cluster/GlobalPolicy/default-policy");
  });

  it("URL-encodes group, namespace, kind, and name", () => {
    expect(
      buildInstanceRoute({
        apiVersion: "ex amp/le.com/v1",
        namespace: "my ns",
        kind: "My Kind",
        name: "my/name",
      })
    ).toBe("/instances/group/ex%20amp/ns/my%20ns/My%20Kind/my%2Fname");
  });

  it("throws when apiVersion is core-group (no slash, e.g. 'v1')", () => {
    // Backend rejects empty apiGroup at the route level (`IsValidAPIGroup`),
    // so a link with `/group//` would always 400. Throw early to surface bugs
    // in callers instead of producing an unusable URL.
    expect(() =>
      buildInstanceRoute({
        apiVersion: "v1",
        namespace: "default",
        kind: "ConfigMap",
        name: "settings",
      })
    ).toThrow(/no apiGroup/);
  });

  it("throws when apiVersion has no slash", () => {
    expect(() =>
      buildInstanceRoute({
        apiVersion: "v1beta1",
        kind: "Thing",
        name: "x",
      })
    ).toThrow(/no apiGroup/);
  });
});

describe("apiGroupOf", () => {
  it.each([
    ["apps.example.com/v1", "apps.example.com"],
    ["v1", ""],
    ["kro.run/v1alpha1", "kro.run"],
    ["", ""],
  ])("apiGroupOf(%j) === %j", (input, expected) => {
    expect(apiGroupOf(input)).toBe(expected);
  });
});
