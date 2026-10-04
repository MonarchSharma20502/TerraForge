// The preview is the generated Terraform tree. It renders from the wasm core's
// output, so what it shows is byte-identical to what the CLI writes.

import { useMemo, useState } from "react";

import type { GenerateResult } from "../core/wasm";

interface PreviewProps {
  result: GenerateResult | null;
  loading: boolean;
  coreError: string | null;
}

export function Preview({ result, loading, coreError }: PreviewProps) {
  const [active, setActive] = useState<string | null>(null);

  const tree = useMemo(
    () => (result ? buildTree(result.order) : []),
    [result],
  );

  const current = active ?? result?.order[0] ?? null;
  const content = current ? result?.files[current] : null;

  if (coreError) {
    return (
      <div className="pane preview">
        <h2 className="pane-title">Preview</h2>
        <p className="preview-error">Could not load the core: {coreError}</p>
      </div>
    );
  }

  if (loading || !result) {
    return (
      <div className="pane preview">
        <h2 className="pane-title">Preview</h2>
        <p className="preview-empty">Loading the generator core...</p>
      </div>
    );
  }

  return (
    <div className="pane preview">
      <h2 className="pane-title">
        Preview
        <span className="preview-count">{result.order.length} files</span>
      </h2>
      {result.error ? (
        <p className="preview-error">{result.error}</p>
      ) : (
        <div className="preview-body">
          <ul className="preview-tree">
            {tree.map((node) => (
              <li
                key={node.path}
                className={`preview-node preview-node-${node.depth}`}
              >
                {node.dir ? (
                  <span className="preview-dir">{node.label}</span>
                ) : (
                  <button
                    className={`preview-file${
                      current === node.path ? " preview-file-active" : ""
                    }`}
                    onClick={() => setActive(node.path)}
                  >
                    {node.label}
                  </button>
                )}
              </li>
            ))}
          </ul>
          <pre className="preview-content">
            <code>{content}</code>
          </pre>
        </div>
      )}
    </div>
  );
}

// TreeNode is one row of the file tree.
interface TreeNode {
  path: string;
  label: string;
  depth: number;
  dir: boolean;
}

// buildTree turns the flat ordered file list into an indented tree. Directories
// are implied by the path segments, so the tree is a pure function of order.
function buildTree(order: string[]): TreeNode[] {
  const out: TreeNode[] = [];
  const seen = new Set<string>();

  for (const path of order) {
    const segments = path.split("/");
    for (let i = 0; i < segments.length - 1; i++) {
      const dirPath = segments.slice(0, i + 1).join("/");
      if (seen.has(dirPath)) continue;
      seen.add(dirPath);
      out.push({
        path: dirPath,
        label: segments[i],
        depth: i,
        dir: true,
      });
    }
    out.push({
      path,
      label: segments[segments.length - 1],
      depth: segments.length - 1,
      dir: false,
    });
  }

  return out;
}
