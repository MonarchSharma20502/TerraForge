// The inspector edits the selected resource instance. Its fields come from the
// catalog entry's required and optional variables, so a new resource kind gets a
// form for free.

import { type CatalogEntry } from "../core/wasm";
import type { Spec } from "../spec/model";

interface InspectorProps {
  spec: Spec;
  catalog: CatalogEntry[];
  selection: { kind: string; id: string } | null;
  onSetProperty: (key: string, value: unknown) => void;
  onRemove: (kind: string, id: string) => void;
}

export function Inspector({
  spec,
  catalog,
  selection,
  onSetProperty,
  onRemove,
}: InspectorProps) {
  if (!selection) {
    return (
      <div className="pane inspector">
        <h2 className="pane-title">Inspector</h2>
        <p className="inspector-empty">Select a resource on the canvas.</p>
      </div>
    );
  }

  const { kind, id } = selection;
  const entry = catalog.find((c) => c.kind === kind);
  const props = spec.resources[kind]?.[id] || {};

  return (
    <div className="pane inspector">
      <h2 className="pane-title">Inspector</h2>
      <div className="inspector-header">
        <span className="inspector-icon">{entry?.icon ?? kind}</span>
        <div>
          <div className="inspector-id">{id}</div>
          <div className="inspector-type">{entry?.azureType ?? kind}</div>
        </div>
        <button
          className="inspector-remove"
          onClick={() => onRemove(kind, id)}
          title={`Remove ${id}`}
        >
          Remove
        </button>
      </div>

      <section className="inspector-section">
        <h3>Required</h3>
        {(entry?.required || []).map((name) => (
          <PropertyField
            key={name}
            name={name}
            value={props[name]}
            required
            onChange={(value) => onSetProperty(name, value)}
          />
        ))}
      </section>

      <section className="inspector-section">
        <h3>Optional</h3>
        {Object.keys(entry?.optional || {}).length === 0 && (
          <p className="inspector-empty">No optional properties.</p>
        )}
        {Object.entries(entry?.optional || {}).map(([name, meta]) => (
          <PropertyField
            key={name}
            name={name}
            value={props[name]}
            hint={meta.description}
            onChange={(value) => onSetProperty(name, value)}
          />
        ))}
      </section>
    </div>
  );
}

// PropertyField renders one editable property. Lists are edited as a
// comma-separated list; maps as a JSON blob; everything else as text.
function PropertyField({
  name,
  value,
  hint,
  required,
  onChange,
}: {
  name: string;
  value: unknown;
  hint?: string;
  required?: boolean;
  onChange: (value: unknown) => void;
}) {
  const isList = Array.isArray(value);
  const isMap =
    typeof value === "object" && value !== null && !Array.isArray(value);

  return (
    <label className="property">
      <span className="property-name">
        {name}
        {required ? <span className="property-required">*</span> : null}
      </span>
      {isList ? (
        <input
          className="property-input"
          value={(value as unknown[]).join(", ")}
          placeholder="comma separated"
          onChange={(event) =>
            onChange(
              event.target.value
                .split(",")
                .map((part) => part.trim())
                .filter((part) => part.length > 0),
            )
          }
        />
      ) : isMap ? (
        <textarea
          className="property-input property-input-map"
          value={JSON.stringify(value, null, 2)}
          rows={3}
          onChange={(event) => {
            try {
              onChange(JSON.parse(event.target.value));
            } catch {
              // Leave the value alone until the JSON parses.
            }
          }}
        />
      ) : (
        <input
          className="property-input"
          value={typeof value === "string" ? value : String(value ?? "")}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {hint ? <span className="property-hint">{hint}</span> : null}
    </label>
  );
}
