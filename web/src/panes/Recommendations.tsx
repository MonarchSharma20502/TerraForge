// The recommendations pane: the analyzer's findings grouped by severity, each
// with the file it came from and the one-line fix. Selection filters the list to
// the node the canvas is focused on.

import { useMemo } from "react";

import type { AnalyzeResult, Finding } from "../core/wasm";

interface RecommendationsProps {
  result: AnalyzeResult | null;
  selection: string | null;
}

export function Recommendations({ result, selection }: RecommendationsProps) {
  const findings = useMemo(() => {
    if (!result) return [];
    if (!selection) return result.findings;
    // A selected node narrows the list to the findings that mention its file.
    const node = result.graph.nodes.find((n) => n.id === selection);
    if (!node) return result.findings;
    return result.findings.filter((f) => f.files?.includes(node.file));
  }, [result, selection]);

  if (!result) return null;

  const critical = findings.filter((f) => f.severity === "critical");

  return (
    <section className="pane recommendations">
      <h2 className="pane-title">
        Recommendations
        <span
          className={`findings-badge ${
            findings.length === 0
              ? "findings-badge-ok"
              : critical.length > 0
                ? "findings-badge-bad"
                : "findings-badge-warn"
          }`}
        >
          {findings.length === 0
            ? "CLEAN"
            : `${critical.length} CRITICAL / ${findings.length - critical.length} WARNING`}
        </span>
      </h2>
      {result.errors.length > 0 ? (
        <ul className="findings-list">
          {result.errors.map((error, index) => (
            <li key={`error-${index}`} className="finding finding-error">
              <div className="finding-rule">parse error</div>
              <div className="finding-message">
                {error.file}
                {error.line > 0 ? `:${error.line}` : ""}: {error.message}
              </div>
            </li>
          ))}
        </ul>
      ) : null}
      {findings.length === 0 ? (
        <p className="findings-empty">
          {result.findings.length === 0
            ? "No recommendations. The code follows the recommended practices."
            : "No recommendations for the selected node."}
        </p>
      ) : (
        <ul className="findings-list">
          {findings.map((finding, index) => (
            <Recommendation key={`${finding.rule}-${index}`} finding={finding} />
          ))}
        </ul>
      )}
    </section>
  );
}

// Recommendation is one finding with its fix.
function Recommendation({ finding }: { finding: Finding }) {
  return (
    <li className={`finding finding-${finding.severity}`}>
      <div className="finding-rule">{finding.rule}</div>
      <div className="finding-message">{finding.message}</div>
      <div className="finding-fix">{finding.fix}</div>
      {finding.files && finding.files.length > 0 ? (
        <div className="finding-files">{finding.files.join(", ")}</div>
      ) : null}
    </li>
  );
}
