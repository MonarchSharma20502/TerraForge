// The single store for the builder. The spec is the source of truth; the
// generated tree, the diagrams and the policy findings are all pure functions
// of it, recomputed by the wasm core on every edit.

import { useCallback, useEffect, useState } from "react";

import { Core, loadCore, type CatalogEntry, type GenerateResult } from "./core/wasm";
import {
  type Spec,
  addResource,
  emptySpec,
  exampleSpec,
  removeResource,
  setProperty,
  toYAML,
} from "./spec/model";

export interface BuilderState {
  // spec is the document being edited.
  spec: Spec;
  // selection is the kind/id pair the inspector edits, or null.
  selection: { kind: string; id: string } | null;
  // catalog drives the palette and the inspector defaults.
  catalog: CatalogEntry[];
  // result is the last generation, or null before the core loads.
  result: GenerateResult | null;
  // coreError is set when the wasm core itself failed to load.
  coreError: string | null;
  // loading is true until the wasm core has instantiated.
  loading: boolean;
}

export function useBuilder(): BuilderState & {
  setMetadata: (key: keyof Spec["metadata"], value: string) => void;
  add: (kind: string) => void;
  addAt: (kind: string, position: { x: number; y: number }) => void;
  move: (kind: string, id: string, position: { x: number; y: number }) => void;
  remove: (kind: string, id: string) => void;
  select: (kind: string, id: string) => void;
  setProp: (key: string, value: unknown) => void;
  loadExample: () => void;
  clear: () => void;
} {
  const [state, setState] = useState<BuilderState>({
    spec: emptySpec(),
    selection: null,
    catalog: [],
    result: null,
    coreError: null,
    loading: true,
  });
  const [core, setCore] = useState<Core | null>(null);

  // Load the wasm core once, then pull the catalog for the palette.
  useEffect(() => {
    let cancelled = false;
    loadCore()
      .then((c) => {
        if (cancelled) return;
        setCore(c);
        setState((s) => ({ ...s, catalog: c.catalog(), loading: false }));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState((s) => ({
          ...s,
          coreError: err instanceof Error ? err.message : String(err),
          loading: false,
        }));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Regenerate whenever the spec changes and the core is ready. This is what
  // makes the preview live: every edit flows through the same pipeline the CLI
  // runs, in the browser, with no backend.
  useEffect(() => {
    if (!core) return;
    const result = core.generate(toYAML(state.spec));
    setState((s) => ({ ...s, result }));
  }, [core, state.spec]);

  const setMetadata = useCallback((key: keyof Spec["metadata"], value: string) => {
    setState((s) => ({
      ...s,
      spec: { ...s.spec, metadata: { ...s.spec.metadata, [key]: value } },
    }));
  }, []);

  const add = useCallback((kind: string) => {
    setState((s) => ({ ...s, spec: addResource(s.spec, kind, s.catalog) }));
  }, []);

  // addAt is the drag-and-drop entry point: the canvas reports where the drop
  // landed so the node appears under the cursor instead of jumping to the next
  // grid slot.
  const addAt = useCallback(
    (kind: string, position: { x: number; y: number }) => {
      setState((s) => ({
        ...s,
        spec: addResource(s.spec, kind, s.catalog, position),
      }));
    },
    [],
  );

  // move persists a node drag so the layout survives regeneration. Positions are
  // stored on the instance, not in a separate layout map, so the spec stays the
  // single source of truth.
  const move = useCallback(
    (kind: string, id: string, position: { x: number; y: number }) => {
      setState((s) => ({ ...s, spec: setProperty(s.spec, kind, id, "_pos", position) }));
    },
    [],
  );

  const remove = useCallback((kind: string, id: string) => {
    setState((s) => {
      const spec = removeResource(s.spec, kind, id);
      const same = s.selection?.kind === kind && s.selection?.id === id;
      return { ...s, spec, selection: same ? null : s.selection };
    });
  }, []);

  const select = useCallback((kind: string, id: string) => {
    setState((s) => ({ ...s, selection: { kind, id } }));
  }, []);

  const setProp = useCallback((key: string, value: unknown) => {
    setState((s) => {
      if (!s.selection) return s;
      const { kind, id } = s.selection;
      return { ...s, spec: setProperty(s.spec, kind, id, key, value) };
    });
  }, []);

  const loadExample = useCallback(() => {
    setState((s) => ({ ...s, spec: exampleSpec(), selection: null }));
  }, []);

  const clear = useCallback(() => {
    setState((s) => ({ ...s, spec: emptySpec(), selection: null }));
  }, []);

  return {
    ...state,
    setMetadata,
    add,
    addAt,
    move,
    remove,
    select,
    setProp,
    loadExample,
    clear,
  };
}
