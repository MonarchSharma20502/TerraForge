// The Auto-nation v1 spec as the UI edits it. This mirrors internal/spec.Spec
// so the document the browser writes is exactly what the Go parser reads.

export interface Spec {
  apiVersion: "autonation/v1";
  metadata: Metadata;
  resources: Record<string, Record<string, ResourceProps>>;
}

export interface Metadata {
  businessUnit: string;
  platform: string;
  environment: string;
  location: string;
  subscriptionId?: string;
  owner?: string;
  tags?: Record<string, string>;
}

// ResourceProps is the per-instance overrides. tags and stack are lifted out by
// the parser; everything else is a kind-specific property.
//
// _pos is the canvas position the UI writes on drag and drop. The leading
// underscore keeps it out of the generated Terraform: the parser only lifts
// known keys, and the YAML emitter drops undefined values.
export type ResourceProps = Record<string, unknown> & {
  tags?: Record<string, string>;
  stack?: string;
  _pos?: { x: number; y: number };
};

export const ENVIRONMENTS = ["Production", "Development", "UAT", "Hub"] as const;
export const LOCATIONS = ["Southeast Asia", "East US"] as const;

// emptySpec is the document the canvas starts from.
export function emptySpec(): Spec {
  return {
    apiVersion: "autonation/v1",
    metadata: {
      businessUnit: "test",
      platform: "platform",
      environment: "Production",
      location: "Southeast Asia",
      subscriptionId: "00000000-0000-0000-0000-000000000000",
      owner: "platform-team",
      tags: { Environment: "Production", Owner: "platform-team" },
    },
    resources: {},
  };
}

// exampleSpec is the document examples/vnet.yaml describes.
export function exampleSpec(): Spec {
  return {
    apiVersion: "autonation/v1",
    metadata: {
      businessUnit: "test",
      platform: "platform",
      environment: "Production",
      location: "Southeast Asia",
      subscriptionId: "00000000-0000-0000-0000-000000000000",
      owner: "platform-team",
      tags: { Environment: "Production", Owner: "platform-team" },
    },
    resources: {
      resource_group: {
        hub: { tags: { Service: "hub" } },
      },
      vnet: {
        spokevnet: { address_space: ["10.0.0.0/16"] },
      },
      subnet: {
        aml: { address_prefixes: ["10.0.1.0/24"] },
      },
    },
  };
}

// addResource inserts an instance of kind, choosing an unused id from the kind's
// default name. position is optional and records where a drop landed so the node
// appears under the cursor.
export function addResource(
  spec: Spec,
  kind: string,
  catalog: CatalogEntry[],
  position?: { x: number; y: number },
): Spec {
  const entry = catalog.find((c) => c.kind === kind);
  const base = entry?.abbr || kind;
  const family = spec.resources[kind] || {};
  let n = 1;
  while (family[`${base}${String(n).padStart(2, "0")}`]) {
    n++;
  }
  const id = `${base}${String(n).padStart(2, "0")}`;
  const props: ResourceProps = {};
  if (position) {
    props._pos = { x: Math.round(position.x), y: Math.round(position.y) };
  }
  return {
    ...spec,
    resources: {
      ...spec.resources,
      [kind]: { ...family, [id]: props },
    },
  };
}

// removeResource deletes an instance.
export function removeResource(
  spec: Spec,
  kind: string,
  id: string,
): Spec {
  const family = { ...spec.resources[kind] };
  delete family[id];
  const resources = { ...spec.resources };
  if (Object.keys(family).length === 0) {
    delete resources[kind];
  } else {
    resources[kind] = family;
  }
  return { ...spec, resources };
}

// setProperty sets one kind-specific property on an instance.
export function setProperty(
  spec: Spec,
  kind: string,
  id: string,
  key: string,
  value: unknown,
): Spec {
  const family = spec.resources[kind] || {};
  const current = family[id] || {};
  const next = { ...current, [key]: value };
  return {
    ...spec,
    resources: { ...spec.resources, [kind]: { ...family, [id]: next } },
  };
}

// toYAML renders the spec as the document the wasm core parses. The core reads
// either JSON or YAML; YAML keeps the editor readable.
export function toYAML(spec: Spec): string {
  return stringify(spec, 0);
}

// stringify is a small YAML emitter for the spec shape. It handles the maps,
// lists and scalars the DSL uses, and indents two spaces per level.
//
// Keys starting with an underscore are UI-only state (the canvas node
// position); they are dropped here so the document the core parses carries
// only what the generator needs.
function stringify(value: unknown, indent: number): string {
  const pad = "  ".repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) return "[]";
    return value
      .map((item) => `${pad}- ${stringifyInline(item, indent + 1)}`)
      .join("\n");
  }
  if (isPlainObject(value)) {
    const keys = Object.keys(value).filter((key) => !key.startsWith("_"));
    if (keys.length === 0) return "{}";
    return keys
      .map((key) => {
        const child = (value as Record<string, unknown>)[key];
        if (isPlainObject(child) && Object.keys(child).length > 0) {
          return `${pad}${key}:\n${stringify(child, indent + 1)}`;
        }
        if (Array.isArray(child) && child.length > 0) {
          return `${pad}${key}:\n${stringify(child, indent + 1)}`;
        }
        return `${pad}${key}: ${stringifyInline(child, indent)}`;
      })
      .join("\n");
  }
  return stringifyInline(value, indent);
}

// stringifyInline renders a scalar or an empty container on one line.
function stringifyInline(value: unknown, _indent: number): string {
  if (value === null || value === undefined) return "null";
  if (typeof value === "string") return needsQuotes(value) ? `"${value}"` : value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (Array.isArray(value)) return value.length === 0 ? "[]" : stringify(value, _indent);
  if (isPlainObject(value)) return Object.keys(value).length === 0 ? "{}" : stringify(value, _indent);
  return String(value);
}

// needsQuotes reports whether a YAML scalar would be ambiguous bare.
function needsQuotes(s: string): boolean {
  return /^[\d-]/.test(s) || /[:#{}[\],&*!|>'"%@`]/.test(s);
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// CatalogEntry is re-declared here so the spec helpers and the palette share one
// type without a circular import into the core bridge.
export interface CatalogEntry {
  kind: string;
  azureType: string;
  abbr: string;
  category: string;
  icon: string;
  stack: string;
  required: string[];
  optional: Record<string, OptionalVar>;
}

// OptionalVar is one module variable that carries a safe default.
export interface OptionalVar {
  type: string;
  default: string;
  description: string;
}
