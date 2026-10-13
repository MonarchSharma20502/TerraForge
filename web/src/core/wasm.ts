// Glue for the Go wasm module. The browser needs Go's wasm_exec.js to
// instantiate the module; it is copied from the Go toolchain at build time.
//
// Usage:
//   const core = await loadCore();
//   const result = core.analyze({ "main.tf": "..." });

// Minimal shape of the functions the wasm module registers on the global.
interface AutonationGlobal {
  autonationAnalyze: (files: string) => string;
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
  if (!g.autonationAnalyze || !g.autonationCatalog) {
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

  // catalog returns the resource-kind catalog the node labels render from.
  catalog(): CatalogEntry[] {
    return JSON.parse(this.global.autonationCatalog());
  }

  // analyze runs the pipeline over a map of path to file content and returns
  // the architecture graph, the diagrams and the recommendations.
  analyze(files: Record<string, string>): AnalyzeResult {
    return JSON.parse(this.global.autonationAnalyze(JSON.stringify(files)));
  }
}

// CatalogEntry is the label-facing projection of one resource kind.
export interface CatalogEntry {
  kind: string;
  azureType: string;
  abbr: string;
  category: string;
  icon: string;
  stack: string;
  required: string[];
}

// AnalyzeResult is the payload the wasm module returns.
export interface AnalyzeResult {
  summary: Summary;
  graph: Graph;
  diagrams: { mermaid: string; d2: string };
  findings: Finding[];
  errors: ParseError[];
  report: string;
  error?: string;
}

// Summary is the headline view of what the code deploys.
export interface Summary {
  resourceCount: number;
  dataCount: number;
  moduleCallCount: number;
  providers: string[];
  byCategory: Record<string, number>;
  byProvider: Record<string, number>;
}

// Graph is the analyzed topology.
export interface Graph {
  nodes: Node[];
  edges: Edge[];
}

// Node is one block in the graph.
export interface Node {
  id: string;
  type: string;
  kind: string;
  name: string;
  blockKind: "resource" | "data" | "module";
  module: string;
  group: string;
  icon: string;
  category: string;
  attributes: Record<string, string>;
  blocks: Record<string, string>;
  file: string;
  line: number;
}

// Edge is a reference from one block to another.
export interface Edge {
  from: string;
  to: string;
  label: string;
  semantic: EdgeSemantic;
}

// EdgeSemantic is the legend class the canvas draws an edge as.
export type EdgeSemantic =
  | "default"
  | "peering"
  | "vnet_integration"
  | "vnet_link"
  | "web_request"
  | "onprem";

// Finding is one recommendation.
export interface Finding {
  rule: string;
  severity: "critical" | "warning";
  message: string;
  fix: string;
  files?: string[];
}

// ParseError is one file that could not be parsed.
export interface ParseError {
  file: string;
  line: number;
  message: string;
}
