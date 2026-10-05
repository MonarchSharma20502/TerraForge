// The palette lists every resource kind the catalog knows, grouped by category.
// It is generated from the catalog data, not hand-built per kind.

import { type CatalogEntry } from "../core/wasm";

interface PaletteProps {
  catalog: CatalogEntry[];
  onAdd: (kind: string) => void;
}

export function Palette({ catalog, onAdd }: PaletteProps) {
  const groups = groupByCategory(catalog);

  return (
    <div className="pane palette">
      <h2 className="pane-title">Resource kinds</h2>
      {groups.map(([category, entries]) => (
        <section key={category} className="palette-group">
          <h3 className="palette-category">{category}</h3>
          {entries.map((entry) => (
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
              <span className="palette-icon">{entry.icon}</span>
              <span className="palette-kind">{entry.kind}</span>
              <span className="palette-abbr">{entry.abbr}</span>
            </button>
          ))}
        </section>
      ))}
    </div>
  );
}

// groupByCategory keeps the palette deterministic: alphabetical within a
// category, categories in the order they first appear.
function groupByCategory(catalog: CatalogEntry[]): [string, CatalogEntry[]][] {
  const order: string[] = [];
  const byCategory = new Map<string, CatalogEntry[]>();
  for (const entry of catalog) {
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
