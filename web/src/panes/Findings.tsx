// The findings panel reports the five policy rules. Download is blocked unless
// every critical finding is clear, so this pane is the gate the UI shows.

import type { GenerateResult } from "../core/wasm";

interface FindingsProps {
  result: GenerateResult | null;
}

export function Findings({ result }: FindingsProps) {
  if (!result) return null;

  const critical = result.policy.findings.filter((f) => f.severity === "critical");

  return (
    <section className="findings">
      <h2 className="pane-title">
        Policy gate
        <span
          className={`findings-badge ${
            result.policy.passed ? "findings-badge-ok" : "findings-badge-bad"
          }`}
        >
          {result.policy.passed ? "PASS" : `${critical.length} BLOCKING`}
        </span>
      </h2>
      {result.policy.findings.length === 0 ? (
        <p className="findings-empty">
          All five rules pass. The tree is ready to download.
        </p>
      ) : (
        <ul className="findings-list">
          {result.policy.findings.map((finding, index) => (
            <li
              key={`${finding.rule}-${index}`}
              className={`finding finding-${finding.severity}`}
            >
              <div className="finding-rule">{finding.rule}</div>
              <div className="finding-message">{finding.message}</div>
              {finding.files && finding.files.length > 0 ? (
                <div className="finding-files">{finding.files.join(", ")}</div>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
