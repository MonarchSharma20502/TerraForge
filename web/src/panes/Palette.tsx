// The palette lists every resource kind the catalog knows, grouped by category.
// It is generated from the catalog data, not hand-built per kind. Groups collapse
// and the list filters, so picking what to place is a short list, not a wall.

import { useMemo, useState } from "react";

import { KindIcon } from "../KindIcon";
import { type CatalogEntry } from "../core/wasm";

interface PaletteProps {
  catalog: CatalogEntry[];
  onAdd: (kind: string) => void;
}

export function Palette({ catalog, onAdd }: PaletteProps) {
  const [filter, setFilter] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const groups = useMemo(
    () => groupByCategory(catalog, filter.trim().toLowerCase()),
    [catalog, filter],
  );

  const shown = groups.reduce((sum, [, entries]) => sum + entries.length, 0);

  return (
    <div className="pane palette">
      <h2 className="pane-title">Resource kinds</h2>
      <input
        className="palette-filter"
        type="search"
        placeholder="Filter resources..."
        value={filter}
        onChange={(event) => setFilter(event.target.value)}
        aria-label="Filter resource kinds"
      />
      {groups.length === 0 ? (
        <p className="palette-empty">
          No resource kind matches &ldquo;{filter}&rdquo;.
        </p>
      ) : (
        groups.map(([category, entries]) => {
          const isCollapsed = collapsed[category] ?? false;
          return (
            <section key={category} className="palette-group">
              <button
                className="palette-category"
                aria-expanded={!isCollapsed}
                onClick={() =>
                  setCollapsed((value) => ({ ...value, [category]: !isCollapsed }))
                }
                title={isCollapsed ? `Expand ${category}` : `Collapse ${category}`}
              >
                <span className="palette-category-name">{category}</span>
                <span className="palette-category-count">{entries.length}</span>
                <span className="palette-category-caret">
                  {isCollapsed ? "▸" : "▾"}
                </span>
              </button>
              {!isCollapsed &&
                entries.map((entry) => (
                  <button
                    key={entry.kind}
                    className="palette-item"
                    draggable
                    onDragStart={(event) =>
                      event.dataTransfer.setData(
                        "application/x-autonation-kind",
                        entry.kind,
                      )
                    }
                    onClick={() => onAdd(entry.kind)}
                    title={`Drag to the canvas, or click to add ${entry.azureType} to the ${entry.stack} stack`}
                  >
                    <span className="palette-icon">
                      <KindIcon kind={entry.kind} />
                    </span>
                    <span className="palette-kind">{entry.kind}</span>
                    <span className="palette-abbr">{entry.abbr}</span>
                  </button>
                ))}
            </section>
          );
        })
      )}
      <p className="palette-summary">{shown} available</p>
    </div>
  );
}

// groupByCategory keeps the palette deterministic: alphabetical within a
// category, categories in the order they first appear. A filter narrows both.
function groupByCategory(
  catalog: CatalogEntry[],
  filter: string,
): [string, CatalogEntry[]][] {
  const order: string[] = [];
  const byCategory = new Map<string, CatalogEntry[]>();
  for (const entry of catalog) {
    if (
      filter &&
      !entry.kind.includes(filter) &&
      !entry.category.toLowerCase().includes(filter)
    ) {
      continue;
    }
    if (!byCategory.has(entry.category)) {
      byCategory.set(entry.category, []);
      order.push(entry.category);
    }
    byCategory.get(entry.category)!.push(entry);
  }
  return order.map((category) => [
    category,
    [...byCategory.get(category)!].sort((a, b) => a.kind.localeCompare(b.kind)),
  ]);
}
