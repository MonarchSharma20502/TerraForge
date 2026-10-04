// Glue for the Go wasm module. The browser needs Go's wasm_exec.js to
// instantiate the module; it is copied from the Go toolchain at build time.
//
// Usage:
//   const core = await loadCore();
//   const result = core.generate(specDocument);

// Minimal shape of the functions the wasm module registers on the global.
interface AutonationGlobal {
  autonationGenerate: (document: string) => string;
  autonationCatalog: () => string;
}

// loadCore instantiates the Go wasm module exactly once and resolves with the
// bridge the UI calls.
export async function loadCore(): Promise<Core> {
  if (!corePromise) {
    corePromise = instantiate();
  }
  return corePromise;
}

let corePromise: Promise<Core> | null = null;

async function instantiate(): Promise<Core> {
  const go = new (window as any).Go();
  // The module lives in public/ so Vite serves it verbatim; the URL is stable
  // at both dev and build time.
  const wasmUrl = new URL("/autonation.wasm", import.meta.url);
  const response = await fetch(wasmUrl);
  const bytes = await response.arrayBuffer();
  const result = await WebAssembly.instantiate(bytes, go.importObject);
  // run() resolves when the wasm module's main() returns; ours blocks forever
  // on select{}, so detach it.
  go.run(result.instance);

  const g = window as unknown as AutonationGlobal;
  if (!g.autonationGenerate || !g.autonationCatalog) {
    throw new Error("autonation wasm did not register its functions");
  }
  return new Core(g);
}

// Core is the typed bridge over the wasm module.
export class Core {
  private global: AutonationGlobal;

  constructor(g: AutonationGlobal) {
    this.global = g;
  }

  // catalog returns the resource-kind catalog the palette renders from.
  catalog(): CatalogEntry[] {
    return JSON.parse(this.global.autonationCatalog());
  }

  // generate runs the whole pipeline and returns the generated tree, the
  // diagrams and the policy findings.
  generate(document: string): GenerateResult {
    return JSON.parse(this.global.autonationGenerate(document));
  }
}

// CatalogEntry is the palette-facing projection of one resource kind.
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

// GenerateResult is the payload the wasm module returns.
export interface GenerateResult {
  files: Record<string, string>;
  order: string[];
  diagrams: { mermaid: string; d2: string };
  policy: PolicyResult;
  error?: string;
}

// PolicyResult is the gate outcome the findings panel renders.
export interface PolicyResult {
  passed: boolean;
  findings: PolicyEntry[];
}

// PolicyEntry is one finding.
export interface PolicyEntry {
  rule: string;
  severity: string;
  message: string;
  files?: string[];
}
