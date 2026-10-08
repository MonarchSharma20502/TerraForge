// The catalog pane: a tabbed, card-based resource browser. Tabs pick the
// category, the dropdown picks the deployment tier, and each card is one
// resource kind you drag or click onto the canvas. It is generated from the
// catalog data, not hand-built per kind.

import { useMemo, useState } from "react";

import { KindIcon } from "../KindIcon";
import { type CatalogEntry } from "../core/wasm";

interface PaletteProps {
  catalog: CatalogEntry[];
  onAdd: (kind: string) => void;
}

export function Palette({ catalog, onAdd }: PaletteProps) {
  const [filter, setFilter] = useState("");
  const [category, setCategory] = useState("All");
  const [stack, setStack] = useState("All");

  const categories = useMemo(() => {
    const seen = new Set<string>();
    catalog.forEach((entry) => seen.add(entry.category));
    return ["All", ...Array.from(seen).sort()];
  }, [catalog]);

  const stacks = useMemo(() => {
    const seen = new Set<string>();
    catalog.forEach((entry) => seen.add(entry.stack));
    return ["All", ...Array.from(seen).sort()];
  }, [catalog]);

  const counts = useMemo(() => {
    const table: Record<string, number> = { All: catalog.length };
    catalog.forEach((entry) => {
      table[entry.category] = (table[entry.category] ?? 0) + 1;
    });
    return table;
  }, [catalog]);

  const entries = useMemo(() => {
    const query = filter.trim().toLowerCase();
    return catalog
      .filter((entry) => category === "All" || entry.category === category)
      .filter((entry) => stack === "All" || entry.stack === stack)
      .filter(
        (entry) =>
          !query ||
          entry.kind.includes(query) ||
          entry.category.toLowerCase().includes(query),
      )
      .sort((a, b) => a.kind.localeCompare(b.kind));
  }, [catalog, category, stack, filter]);

  return (
    <div className="pane palette">
      <h2 className="pane-title">Resource catalog</h2>
      <div
        className="palette-tabs"
        role="tablist"
        aria-label="Resource category"
      >
        {categories.map((name) => (
          <button
            key={name}
            type="button"
            role="tab"
            aria-selected={category === name}
            className={category === name ? "palette-tab active" : "palette-tab"}
            onClick={() => setCategory(name)}
          >
            {name}
            <span className="palette-tab-count">{counts[name] ?? 0}</span>
          </button>
        ))}
      </div>
      <div className="palette-controls">
        <label className="palette-select-label">
          <span>Tier</span>
          <select
            className="palette-select"
            value={stack}
            onChange={(event) => setStack(event.target.value)}
            aria-label="Deployment tier"
          >
            {stacks.map((name) => (
              <option key={name} value={name}>
                {name === "All" ? "All tiers" : `${name} tier`}
              </option>
            ))}
          </select>
        </label>
        <input
          className="palette-filter"
          type="search"
          placeholder="Search..."
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
          aria-label="Search resource kinds"
        />
      </div>
      {entries.length === 0 ? (
        <p className="palette-empty">
          No resource kind matches &ldquo;{filter}&rdquo;.
        </p>
      ) : (
        <div className="palette-grid">
          {entries.map((entry) => (
            <button
              key={entry.kind}
              type="button"
              className="palette-card"
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
              <span className="palette-card-icon">
                <KindIcon kind={entry.kind} />
              </span>
              <span className="palette-card-text">
                <span className="palette-card-name">{entry.kind}</span>
                <span className="palette-card-type">{entry.abbr}</span>
              </span>
              <span className="palette-card-add" aria-hidden="true">
                +
              </span>
            </button>
          ))}
        </div>
      )}
      <p className="palette-summary">
        {entries.length} of {catalog.length} available
      </p>
    </div>
  );
}
