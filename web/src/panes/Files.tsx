// The input pane: the list of Terraform files being analyzed plus the content of
// the selected one. Files are added by folder drop or file picker, and edited in
// place so a fix can be re-analyzed without leaving the browser.

import { useEffect, useMemo, useState } from "react";

interface FilesProps {
  files: Record<string, string>;
  selection: string | null;
  onAddFiles: (files: Record<string, string>) => void;
  onEdit: (path: string, content: string) => void;
  onRemove: (path: string) => void;
  onSelectFile: (path: string) => void;
}

export function Files({
  files,
  selection,
  onAddFiles,
  onEdit,
  onRemove,
  onSelectFile,
}: FilesProps) {
  const paths = useMemo(() => Object.keys(files).sort(), [files]);
  const [active, setActive] = useState<string | null>(null);

  const current = active ?? paths[0] ?? null;

  // Keep a local draft of the file being edited so typing does not fight the
  // re-analysis on every keystroke.
  const [draft, setDraft] = useState<string>("");
  useEffect(() => {
    setDraft(current ? files[current] ?? "" : "");
  }, [current, files]);

  const handleDrop = (event: React.DragEvent) => {
    event.preventDefault();
    const incoming: Record<string, string> = {};
    const list = event.dataTransfer.items
      ? Array.from(event.dataTransfer.items)
      : [];
    const pending: Promise<void>[] = [];
    list.forEach((item) => {
      const entry = item.webkitGetAsEntry?.();
      if (!entry) return;
      pending.push(walkEntry(entry, "", incoming));
    });
    Promise.all(pending).then(() => {
      if (Object.keys(incoming).length > 0) onAddFiles(incoming);
    });
  };

  const handlePick = async () => {
    const input = document.createElement("input");
    input.type = "file";
    input.multiple = true;
    // .tf and .tf.json are not in the accept spec's known list, so accept any
    // file and filter on extension when reading.
    input.accept = ".tf,.tf.json,text/plain";
    const chosen = await new Promise<FileList | null>((resolve) => {
      input.onchange = () => resolve(input.files);
      input.click();
    });
    if (!chosen) return;
    const incoming: Record<string, string> = {};
    for (const file of Array.from(chosen)) {
      if (!file.name.endsWith(".tf") && !file.name.endsWith(".tf.json")) {
        continue;
      }
      incoming[file.name] = await file.text();
    }
    if (Object.keys(incoming).length > 0) onAddFiles(incoming);
  };

  return (
    <div className="pane files">
      <h2 className="pane-title">
        Terraform input
        <span className="files-count">{paths.length} file(s)</span>
        <button className="files-add" onClick={handlePick} title="Add .tf files">
          Add files
        </button>
      </h2>
      {paths.length === 0 ? (
        <div
          className="files-dropzone"
          onDragOver={(event) => event.preventDefault()}
          onDrop={handleDrop}
        >
          <p className="files-empty">
            Drop a folder of Terraform here, add files, or load the example.
          </p>
        </div>
      ) : (
        <div className="files-body">
          <ul
            className="files-list"
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
          >
            {paths.map((path) => (
              <li key={path} className="files-item">
                <button
                  className={`files-file${
                    current === path ? " files-file-active" : ""
                  }`}
                  onClick={() => {
                    setActive(path);
                    onSelectFile(path);
                  }}
                >
                  {path}
                </button>
                <button
                  className="files-remove"
                  onClick={() => {
                    onRemove(path);
                    if (current === path) setActive(null);
                  }}
                  title={`Remove ${path}`}
                >
                  ×
                </button>
              </li>
            ))}
          </ul>
          {current ? (
            <div className="files-editor">
              <div className="files-editor-bar">
                <span className="files-editor-name">{current}</span>
                <button
                  className="files-editor-save"
                  onClick={() => onEdit(current, draft)}
                  title="Re-analyze with this content"
                >
                  Re-analyze
                </button>
              </div>
              <textarea
                className="files-editor-text"
                spellCheck={false}
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
              />
            </div>
          ) : null}
        </div>
      )}
      {selection ? <p className="files-hint">Selected: {selection}</p> : null}
    </div>
  );
}

// walkEntry reads a dropped FileSystemEntry, recursing into directories and
// reading every .tf and .tf.json file it finds.
function walkEntry(
  entry: FileSystemEntry,
  prefix: string,
  out: Record<string, string>,
): Promise<void> {
  if (entry.isFile) {
    return new Promise((resolve) => {
      (entry as FileSystemFileEntry).file((file) => {
        if (file.name.endsWith(".tf") || file.name.endsWith(".tf.json")) {
          file.text().then((text) => {
            out[prefix + file.name] = text;
            resolve();
          });
          return;
        }
        resolve();
      });
    });
  }
  if (entry.isDirectory) {
    return new Promise((resolve) => {
      const reader = (entry as FileSystemDirectoryEntry).createReader();
      const readAll: Promise<void>[] = [];
      const step = () => {
        reader.readEntries((entries) => {
          if (entries.length === 0) {
            Promise.all(readAll).then(() => resolve());
            return;
          }
          entries.forEach((child) => {
            readAll.push(
              walkEntry(child, prefix + entry.name + "/", out),
            );
          });
          step();
        });
      };
      step();
    });
  }
  return Promise.resolve();
}
