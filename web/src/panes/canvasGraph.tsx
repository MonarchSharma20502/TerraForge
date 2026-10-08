import type { Edge, Node } from "@xyflow/react";

import type { Spec } from "../spec/model";
import type { CatalogEntry } from "../core/wasm";
import { KindIcon } from "../KindIcon";

// The NOC governance template pins a colour per architecture tier and a style
// per connector type. Keeping them in one place keeps the canvas and the legend
// in sync.
const TIER_CLASS: Record<string, string> = {
  Core: "tier-core",
  Network: "tier-network",
  Data: "tier-data",
  Compute: "tier-compute",
};

const TIER_LABEL: Record<string, string> = {
  Core: "Core",
  Network: "Network",
  Data: "Data",
  Compute: "Compute",
};

// The template's legend colours: peering and VNet integration are cyan, a web
// request is green, an on-prem tunnel is black, a VNet link is blue.
const EDGE_STYLE: Record<string, string> = {
  peering: "edge-peering",
  vnet_integration: "edge-peering",
  web_request: "edge-web-request",
  onprem: "edge-onprem",
  vnet_link: "edge-vnet-link",
  default: "edge-default",
};

// toGraph turns the spec into React Flow nodes and edges. Node ids carry the
// kind and the instance id separated by :: so a click can restore both.
export function toGraph(
  spec: Spec,
  catalog: CatalogEntry[],
  selection: { kind: string; id: string } | null,
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  // Stack columns so the three-tier layout reads left to right. The pitch is
  // the node width plus its gutter, so columns never overlap.
  const NODE_W = 124;
  const PITCH = 150;
  const stackX: Record<string, number> = {};
  const stackCounts: Record<string, number> = {};

  Object.entries(spec.resources).forEach(([kind, family]) => {
    Object.entries(family).forEach(([id, props]) => {
      const entry = catalog.find((c) => c.kind === kind);
      const stack = (props.stack as string) || entry?.stack || "Core";
      const column = stackX[stack] ?? Object.keys(stackX).length * PITCH;
      const row = stackCounts[stack] ?? 0;
      stackCounts[stack] = row + 1;
      if (!(stack in stackX)) stackX[stack] = column;

      // A position the user dragged to wins over the grid slot.
      const pos = props._pos as { x: number; y: number } | undefined;
      nodes.push({
        id: `${kind}::${id}`,
        position: pos ?? { x: column, y: row * 96 },
        width: NODE_W,
        data: {
          label: (
            <div className="canvas-node">
              <span className="canvas-node-icon">
                <KindIcon kind={kind} />
              </span>
              <span className="canvas-node-id">{id}</span>
              <span className="canvas-node-kind">{kind}</span>
            </div>
          ),
        },
        className: [
          "canvas-node-wrap",
          TIER_CLASS[stack] ?? "tier-core",
          selection?.kind === kind && selection?.id === id
            ? "canvas-node-selected"
            : "",
        ]
          .filter(Boolean)
          .join(" "),
      });
    });
  });

  // One grouped container per stack, so the canvas reads as subscription /
  // resource-group boundaries rather than a flat field of nodes. The template
  // wants each boundary drawn at a consistent weight with its label top-left.
  Object.keys(stackX).forEach((stack) => {
    const count = stackCounts[stack] ?? 0;
    nodes.push({
      id: `stack::${stack}`,
      position: { x: stackX[stack] - 18, y: -48 },
      // The container only has to be wide enough for its column and tall
      // enough for every node plus the label band.
      width: NODE_W + 30,
      height: count * 96 + 66,
      data: {
        label: (
          <div className="canvas-stack">
            <span className="canvas-stack-name">{TIER_LABEL[stack] ?? stack}</span>
          </div>
        ),
      },
      className: `canvas-stack-wrap ${TIER_CLASS[stack] ?? "tier-core"}`,
      // A container must not steal clicks from the resources inside it.
      selectable: false,
      draggable: false,
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
          className: EDGE_STYLE[edgeKind(kind, dep)] ?? EDGE_STYLE.default,
        });
      });
    });
  });

  return { nodes, edges };
}

// edgeKind maps a dependency to the template legend it should be drawn as. A
// subnet or NIC dependency inside a virtual network is a VNet link; a peering
// or integration dependency is the cyan connector; anything else is a plain
// dependency arrow.
function edgeKind(kind: string, dep: string): string {
  const depKind = dep.split("::")[0];
  if (kind === "vnet" && depKind === "vnet") return "peering";
  if (kind === "private_dns_zone" && depKind === "vnet") return "vnet_link";
  if (depKind === "subnet" || depKind === "vnet") return "vnet_integration";
  if (kind === "windows_vm" || kind === "linux_vm") return "web_request";
  return "default";
}
