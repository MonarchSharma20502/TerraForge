// The canvas is the React Flow graph of the spec. Nodes are resource instances
// grouped by their Workload stack; edges are the dependency relationships the
// resolver computed.

import { useMemo } from "react";
import {
  Background,
  Controls,
  type Edge,
  type Node,
  ReactFlow,
  type NodeMouseHandler,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { Spec } from "../spec/model";
import type { CatalogEntry } from "../core/wasm";

interface CanvasProps {
  spec: Spec;
  catalog: CatalogEntry[];
  selection: { kind: string; id: string } | null;
  onSelect: (kind: string, id: string) => void;
}

export function Canvas({ spec, catalog, selection, onSelect }: CanvasProps) {
  const { nodes, edges } = useMemo(
    () => toGraph(spec, catalog, selection),
    [spec, catalog, selection],
  );

  const handleNodeClick: NodeMouseHandler = (_event, node) => {
    const [kind, id] = node.id.split("::");
    onSelect(kind, id);
  };

  return (
    <div className="pane canvas">
      <h2 className="pane-title">Architecture</h2>
      <div className="canvas-flow">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodeClick={handleNodeClick}
          fitView
          nodesDraggable
          proOptions={{ hideAttribution: true }}
        >
          <Background />
          <Controls showInteractive={false} />
        </ReactFlow>
      </div>
    </div>
  );
}

// toGraph turns the spec into React Flow nodes and edges. Node ids carry the
// kind and the instance id separated by :: so a click can restore both.
function toGraph(
  spec: Spec,
  catalog: CatalogEntry[],
  selection: { kind: string; id: string } | null,
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  // Stack columns so the three-tier layout reads left to right.
  const stackX: Record<string, number> = {};
  const stackCounts: Record<string, number> = {};

  Object.entries(spec.resources).forEach(([kind, family]) => {
    Object.entries(family).forEach(([id, props]) => {
      const entry = catalog.find((c) => c.kind === kind);
      const stack = (props.stack as string) || entry?.stack || "Core";
      const column = stackX[stack] ?? Object.keys(stackX).length * 260;
      const row = stackCounts[stack] ?? 0;
      stackCounts[stack] = row + 1;
      if (!(stack in stackX)) stackX[stack] = column;

      nodes.push({
        id: `${kind}::${id}`,
        position: { x: column, y: row * 110 },
        data: {
          label: (
            <div className="canvas-node">
              <span className="canvas-node-icon">{entry?.icon ?? kind}</span>
              <span className="canvas-node-id">{id}</span>
              <span className="canvas-node-kind">{kind}</span>
            </div>
          ),
        },
        className:
          selection?.kind === kind && selection?.id === id
            ? "canvas-node-selected"
            : "",
      });
    });
  });

  // Edges follow the resolver's dependency order: a resource that depends on
  // another points at it.
  Object.entries(spec.resources).forEach(([kind, family]) => {
    Object.entries(family).forEach(([id, props]) => {
      const dependsOn = (props.dependsOn as string[]) || [];
      dependsOn.forEach((dep) => {
        edges.push({
          id: `${kind}::${id}->${dep}`,
          source: dep,
          target: `${kind}::${id}`,
        });
      });
    });
  });

  return { nodes, edges };
}
