// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from "vitest";
import { buildTabsFromSchema } from "./build-tabs";
import type { FormSchema, FormProperty } from "@/types/rgd";

const baseSchema: Omit<FormSchema, "properties"> = {
  name: "demo",
  namespace: "default",
  group: "demo.knodex.io",
  kind: "Demo",
  version: "v1alpha1",
};

function makeSchema(overrides: Partial<FormSchema>): FormSchema {
  return { ...baseSchema, properties: {}, ...overrides };
}

describe("buildTabsFromSchema", () => {
  it("returns [general, review] for empty/null schema", () => {
    expect(buildTabsFromSchema(null).map((t) => t.id)).toEqual([
      "general",
      "review",
    ]);
    expect(buildTabsFromSchema(undefined).map((t) => t.id)).toEqual([
      "general",
      "review",
    ]);
    expect(
      buildTabsFromSchema(makeSchema({ properties: {} })).map((t) => t.id)
    ).toEqual(["general", "review"]);
  });

  it("General is always the first tab", () => {
    const schema = makeSchema({
      properties: {
        replicas: { type: "integer" },
        networking: {
          type: "object",
          properties: { port: { type: "integer" } },
        },
      },
    });
    const tabs = buildTabsFromSchema(schema);
    expect(tabs[0].id).toBe("general");
    expect(tabs[0].kind).toBe("general");
  });

  it("sweeps scalars into the General tab", () => {
    const schema = makeSchema({
      properties: {
        name: { type: "string" },
        replicas: { type: "integer" },
      },
    });
    const tabs = buildTabsFromSchema(schema);
    expect(tabs.map((t) => t.id)).toEqual(["general", "review"]);
    expect(Object.keys(tabs[0].properties ?? {}).sort()).toEqual([
      "name",
      "replicas",
    ]);
  });

  it("emits one tab per object key alongside an empty General tab", () => {
    const networking: FormProperty = {
      type: "object",
      properties: { port: { type: "integer" } },
    };
    const storage: FormProperty = {
      type: "object",
      properties: { size: { type: "string" } },
    };
    const schema = makeSchema({
      properties: { networking, storage },
    });
    expect(buildTabsFromSchema(schema).map((t) => t.id)).toEqual([
      "general",
      "networking",
      "storage",
      "review",
    ]);
  });

  it("mixes scalars and object tabs", () => {
    const schema = makeSchema({
      properties: {
        name: { type: "string" },
        networking: {
          type: "object",
          properties: { port: { type: "integer" } },
        },
      },
    });
    expect(buildTabsFromSchema(schema).map((t) => t.id)).toEqual([
      "general",
      "networking",
      "review",
    ]);
  });

  it("respects propertyOrder for object tabs (listed first, unlisted alphabetized)", () => {
    const schema = makeSchema({
      properties: {
        a: { type: "object", properties: { x: { type: "string" } } },
        b: { type: "object", properties: { x: { type: "string" } } },
        c: { type: "object", properties: { x: { type: "string" } } },
      },
      propertyOrder: ["b", "a"],
    });
    expect(buildTabsFromSchema(schema).map((t) => t.id)).toEqual([
      "general",
      "b",
      "a",
      "c",
      "review",
    ]);
  });

  it("treats 'advanced' object as its own tab (same as any other object property)", () => {
    const schema = makeSchema({
      properties: {
        advanced: {
          type: "object",
          properties: { mtls: { type: "boolean" } },
        },
        networking: {
          type: "object",
          properties: { port: { type: "integer" } },
        },
      },
    });
    const ids = buildTabsFromSchema(schema).map((t) => t.id);
    expect(ids).toEqual(["general", "advanced", "networking", "review"]);
  });

  it("filters reserved Knodex plumbing keys from the General tab properties", () => {
    const schema = makeSchema({
      properties: {
        namespace: { type: "string" },
        project: { type: "string" },
        instanceName: { type: "string" },
        replicas: { type: "integer" },
      },
    });
    const tabs = buildTabsFromSchema(schema);
    const general = tabs.find((t) => t.id === "general");
    expect(general).toBeDefined();
    const generalKeys = Object.keys(general?.properties ?? {});
    expect(generalKeys).toEqual(["replicas"]);
    expect(generalKeys).not.toContain("namespace");
    expect(generalKeys).not.toContain("project");
    expect(generalKeys).not.toContain("instanceName");
  });

  it("prefixes colliding object tab ids with 'rgd-'", () => {
    const schema = makeSchema({
      properties: {
        general: {
          type: "object",
          properties: { foo: { type: "string" } },
        },
        review: {
          type: "object",
          properties: { foo: { type: "string" } },
        },
      },
    });
    const ids = buildTabsFromSchema(schema).map((t) => t.id);
    expect(ids).toEqual(["general", "rgd-general", "rgd-review", "review"]);
  });

  it("populates required from prop.required for object tabs and filters General required to general keys", () => {
    const networking: FormProperty = {
      type: "object",
      properties: {
        port: { type: "integer" },
        host: { type: "string" },
      },
      required: ["port"],
    };
    const schema = makeSchema({
      properties: {
        name: { type: "string" },
        replicas: { type: "integer" },
        networking,
      },
      // schema.required contains both scalar AND object keys — object should be filtered out for General.
      required: ["name", "networking"],
    });
    const tabs = buildTabsFromSchema(schema);
    const general = tabs.find((t) => t.id === "general");
    const networkingTab = tabs.find((t) => t.id === "networking");
    expect(general?.required).toEqual(["name"]);
    expect(networkingTab?.required).toEqual(["port"]);
  });

  it("includes 'advanced' scalar in the General tab like any other scalar", () => {
    const schema = makeSchema({
      properties: {
        advanced: { type: "boolean" },
        replicas: { type: "integer" },
      },
    });
    const ids = buildTabsFromSchema(schema).map((t) => t.id);
    expect(ids).toEqual(["general", "review"]);
    const general = buildTabsFromSchema(schema).find((t) => t.id === "general");
    expect(Object.keys(general?.properties ?? {})).toEqual([
      "advanced",
      "replicas",
    ]);
  });

  it("labels object tabs with prop.title when set, else formatLabel(key)", () => {
    const schema = makeSchema({
      properties: {
        networking: {
          type: "object",
          title: "Network Config",
          properties: { port: { type: "integer" } },
        },
        databaseConnection: {
          type: "object",
          properties: { host: { type: "string" } },
        },
      },
    });
    const tabs = buildTabsFromSchema(schema);
    expect(tabs.find((t) => t.id === "networking")?.label).toBe(
      "Network Config"
    );
    expect(tabs.find((t) => t.id === "databaseConnection")?.label).toBe(
      "Database Connection"
    );
  });

  // --- externalRef folding (top-level → General, nested → owning tab) ---

  it("folds top-level externalRef object into the General tab (no separate tab)", () => {
    const externalRef: FormProperty = {
      type: "object",
      properties: {
        cluster: {
          type: "object",
          properties: {
            name: { type: "string" },
            namespace: { type: "string" },
          },
        },
      },
    };
    const schema = makeSchema({
      properties: {
        externalRef,
        minNodes: { type: "integer" },
      },
    });
    const tabs = buildTabsFromSchema(schema);
    expect(tabs.map((t) => t.id)).toEqual(["general", "review"]);
    const general = tabs.find((t) => t.id === "general");
    expect(general?.properties).toBeDefined();
    expect(Object.keys(general?.properties ?? {}).sort()).toEqual([
      "externalRef",
      "minNodes",
    ]);
    expect(general?.properties?.externalRef.type).toBe("object");
  });

  it("does NOT create an 'externalRef' tab even when it's the only top-level property", () => {
    const schema = makeSchema({
      properties: {
        externalRef: {
          type: "object",
          properties: {
            cluster: {
              type: "object",
              properties: {
                name: { type: "string" },
                namespace: { type: "string" },
              },
            },
          },
        },
      },
    });
    expect(buildTabsFromSchema(schema).map((t) => t.id)).toEqual([
      "general",
      "review",
    ]);
  });

  it("keeps nested externalRef under its owning object tab (not folded into General)", () => {
    const db: FormProperty = {
      type: "object",
      properties: {
        host: { type: "string" },
        externalRef: {
          type: "object",
          properties: {
            secret: {
              type: "object",
              properties: {
                name: { type: "string" },
                namespace: { type: "string" },
              },
            },
          },
        },
      },
    };
    const schema = makeSchema({
      properties: { db, replicas: { type: "integer" } },
    });
    const tabs = buildTabsFromSchema(schema);
    expect(tabs.map((t) => t.id)).toEqual(["general", "db", "review"]);

    const general = tabs.find((t) => t.id === "general");
    // Nested externalRef stays where it is — not surfaced at the General level.
    expect(Object.keys(general?.properties ?? {})).toEqual(["replicas"]);
    expect(general?.properties?.externalRef).toBeUndefined();

    // It remains inside the `db` tab's properties.
    const dbTab = tabs.find((t) => t.id === "db");
    expect(Object.keys(dbTab?.properties ?? {}).sort()).toEqual([
      "externalRef",
      "host",
    ]);
  });

  it("respects propertyOrder when externalRef is folded into General", () => {
    const schema = makeSchema({
      properties: {
        externalRef: {
          type: "object",
          properties: {
            cluster: {
              type: "object",
              properties: { name: { type: "string" } },
            },
          },
        },
        minNodes: { type: "integer" },
        maxNodes: { type: "integer" },
      },
      propertyOrder: ["externalRef", "minNodes", "maxNodes"],
    });
    const general = buildTabsFromSchema(schema).find((t) => t.id === "general");
    expect(general?.propertyOrder).toEqual([
      "externalRef",
      "minNodes",
      "maxNodes",
    ]);
  });
});
