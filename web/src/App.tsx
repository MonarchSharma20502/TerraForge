// The builder shell: metadata bar, palette, canvas, inspector, preview and the
// policy findings panel. Every pane is a pure function of the spec.

import { Canvas } from "./panes/Canvas";
import { Findings } from "./panes/Findings";
import { Inspector } from "./panes/Inspector";
import { Palette } from "./panes/Palette";
import { Preview } from "./panes/Preview";
import { useBuilder } from "./store";
import { ENVIRONMENTS, LOCATIONS, type Spec } from "./spec/model";

export function App() {
  const builder = useBuilder();

  return (
    <div className="app">
      <header className="bar">
        <h1 className="bar-title">Auto-nation</h1>
        <div className="bar-fields">
          <label className="bar-field">
            <span>Business unit</span>
            <input
              value={builder.spec.metadata.businessUnit}
              onChange={(event) =>
                builder.setMetadata("businessUnit", event.target.value)
              }
            />
          </label>
          <label className="bar-field">
            <span>Platform</span>
            <input
              value={builder.spec.metadata.platform}
              onChange={(event) =>
                builder.setMetadata("platform", event.target.value)
              }
            />
          </label>
          <label className="bar-field">
            <span>Environment</span>
            <select
              value={builder.spec.metadata.environment}
              onChange={(event) =>
                builder.setMetadata("environment", event.target.value)
              }
            >
              {ENVIRONMENTS.map((env) => (
                <option key={env} value={env}>
                  {env}
                </option>
              ))}
            </select>
          </label>
          <label className="bar-field">
            <span>Location</span>
            <select
              value={builder.spec.metadata.location}
              onChange={(event) =>
                builder.setMetadata("location", event.target.value)
              }
            >
              {LOCATIONS.map((loc) => (
                <option key={loc} value={loc}>
                  {loc}
                </option>
              ))}
            </select>
          </label>
          <label className="bar-field">
            <span>Owner</span>
            <input
              value={builder.spec.metadata.owner ?? ""}
              onChange={(event) =>
                builder.setMetadata("owner", event.target.value)
              }
            />
          </label>
        </div>
        <div className="bar-actions">
          <button onClick={builder.loadExample}>Load example</button>
          <button onClick={builder.clear}>Clear</button>
        </div>
      </header>

      <main className="grid">
        <Palette catalog={builder.catalog} onAdd={builder.add} />
        <Canvas
          spec={builder.spec}
          catalog={builder.catalog}
          selection={builder.selection}
          onSelect={builder.select}
        />
        <Inspector
          spec={builder.spec}
          catalog={builder.catalog}
          selection={builder.selection}
          onSetProperty={builder.setProp}
          onRemove={builder.remove}
        />
        <Preview
          result={builder.result}
          loading={builder.loading}
          coreError={builder.coreError}
        />
      </main>

      <footer className="gate">
        <Findings result={builder.result} />
      </footer>
    </div>
  );
}

// Kept so the metadata field keys stay type-checked against the spec.
export type { Spec };
