// The canvas is the React Flow graph of the spec. Nodes are resource instances
// grouped by their Workload stack; edges are the dependency relationships the
// resolver computed.
//
// The pane expands to a full-viewport overlay on demand: an architecture does
// not fit in a 300px column. Palette items and the canvas itself both accept
// drops, so a kind dragged from the palette lands under the cursor.

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Background,
  Controls,
  type Node,
  type NodeMouseHandler,
  type OnConnect,
  ReactFlow,
  addEdge,
  useEdgesState,
  useNodesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { Spec } from "../spec/model";
import type { CatalogEntry } from "../core/wasm";
import { toGraph } from "./canvasGraph";

interface CanvasProps {
  spec: Spec;
  catalog: CatalogEntry[];
  selection: { kind: string; id: string } | null;
  onSelect: (kind: string, id: string) => void;
  onAddAt: (kind: string, position: { x: number; y: number }) => void;
  onMove: (kind: string, id: string, position: { x: number; y: number }) => void;
}

export function Canvas({
  spec,
  catalog,
  selection,
  onSelect,
  onAddAt,
  onMove,
}: CanvasProps) {
  const [expanded, setExpanded] = useState(false);
  const { nodes, edges } = useMemo(
    () => toGraph(spec, catalog, selection),
    [spec, catalog, selection],
  );

  // React Flow owns the drag state; the spec is only updated when a drag ends so
  // the layout survives regeneration without fighting the pointer. useNodesState
  // only seeds from its argument, so a new spec has to be applied explicitly or
  // the canvas stays on whatever it first rendered.
  const [flowNodes, setFlowNodes, onNodesChange] = useNodesState(nodes);
  const [flowEdges, setFlowEdges, onEdgesChange] = useEdgesState(edges);

  useEffect(() => {
    setFlowNodes(nodes);
  }, [nodes, setFlowNodes]);

  useEffect(() => {
    setFlowEdges(edges);
  }, [edges, setFlowEdges]);

  const handleNodeClick: NodeMouseHandler = (_event, node) => {
    const [kind, id] = node.id.split("::");
    onSelect(kind, id);
  };

  const handleNodeDragStop = (_event: unknown, node: Node) => {
    const [kind, id] = node.id.split("::");
    onMove(kind, id, node.position);
  };

  // A palette item is dragged over the surface. React Flow reports the position
  // in its own coordinate space, which is what the node consumes.
  const handleDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const kind = event.dataTransfer.getData("application/x-autonation-kind");
      if (!kind) return;
      const bounds = event.currentTarget.getBoundingClientRect();
      onAddAt(kind, {
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });
    },
    [onAddAt],
  );

  const handleDragOver = useCallback((event: React.DragEvent) => {
    // preventDefault marks the surface as a drop target; without it the browser
    // navigates instead of dropping.
    event.preventDefault();
    event.dataTransfer.dropEffect = "copy";
  }, []);

  const handleConnect: OnConnect = useCallback(
    (connection) => {
      setFlowEdges((eds) => addEdge(connection, eds));
    },
    [setFlowEdges],
  );

  const flow = (
    <ReactFlow
      nodes={flowNodes}
      edges={flowEdges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onNodeClick={handleNodeClick}
      onNodeDragStop={handleNodeDragStop}
      onConnect={handleConnect}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      fitView
      nodesDraggable
      proOptions={{ hideAttribution: true }}
    >
      <Background />
      <Controls showInteractive={false} />
    </ReactFlow>
  );

  return (
    <div className={`pane canvas${expanded ? " canvas-expanded" : ""}`}>
      <h2 className="pane-title">
        Architecture
        <button
          className="canvas-expand"
          onClick={() => setExpanded((value) => !value)}
          title={expanded ? "Collapse the architecture" : "Expand the architecture"}
        >
          {expanded ? "Exit full screen" : "Expand"}
        </button>
      </h2>
      <div className="canvas-flow">{flow}</div>
      {expanded && <div className="canvas-overlay">{flow}</div>}
    </div>
    );
}
