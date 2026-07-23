import { EditorView, keymap, lineNumbers, highlightActiveLineGutter, highlightActiveLine, drawSelection, dropCursor } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { cpp } from "@codemirror/lang-cpp";
import { json } from "@codemirror/lang-json";

export class CodeEditor {
  private view: EditorView | null = null;
  private currentPath: string | null = null;
  private isDirty: boolean = false;
  private onDirtyChange: ((dirty: boolean) => void) | null = null;

  private theme = EditorView.theme({
    "&": {
      height: "100%",
      backgroundColor: "var(--bg-inset)",
      color: "var(--text-primary)",
      fontFamily: "var(--font-mono)",
      fontSize: "13px"
    },
    ".cm-content": {
      caretColor: "var(--accent)",
      padding: "8px 0"
    },
    ".cm-cursor, .cm-dropCursor": {
      borderLeftColor: "var(--accent)"
    },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
      backgroundColor: "var(--accent-dim) !important"
    },
    ".cm-panels": {
      backgroundColor: "var(--bg-surface)",
      color: "var(--text-primary)"
    },
    ".cm-gutters": {
      backgroundColor: "var(--bg-surface)",
      color: "var(--text-tertiary)",
      borderRight: "1px solid var(--border-hairline)"
    },
    ".cm-activeLineGutter": {
      backgroundColor: "var(--bg-surface-raised)",
      color: "var(--text-primary)"
    },
    ".cm-activeLine": {
      backgroundColor: "rgba(255, 255, 255, 0.03)"
    },
    ".cm-lineNumbers .cm-gutterElement": {
      padding: "0 8px"
    }
  }, { dark: true });

  public mount(container: HTMLElement, onDirtyChange?: (dirty: boolean) => void): void {
    this.onDirtyChange = onDirtyChange || null;

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        if (!this.isDirty) {
          this.isDirty = true;
          if (this.onDirtyChange) this.onDirtyChange(true);
        }
      }
    });

    const startState = EditorState.create({
      doc: "// PyxForge Native Code Editor\n// Select a file from the workspace explorer on the left to begin editing.\n",
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        drawSelection(),
        dropCursor(),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        cpp(),
        this.theme,
        updateListener
      ]
    });

    this.view = new EditorView({
      state: startState,
      parent: container
    });
  }

  public openFile(path: string, content: string): void {
    this.currentPath = path;
    this.isDirty = false;
    if (this.onDirtyChange) this.onDirtyChange(false);

    if (!this.view) return;

    let languageExtension = [];
    if (path.endsWith(".c") || path.endsWith(".h") || path.endsWith(".cpp") || path.endsWith(".asm")) {
      languageExtension.push(cpp());
    } else if (path.endsWith(".json") || path.endsWith(".toml")) {
      languageExtension.push(json());
    }

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        if (!this.isDirty) {
          this.isDirty = true;
          if (this.onDirtyChange) this.onDirtyChange(true);
        }
      }
    });

    const newState = EditorState.create({
      doc: content,
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        drawSelection(),
        dropCursor(),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        ...languageExtension,
        this.theme,
        updateListener
      ]
    });

    this.view.setState(newState);
  }

  public getContent(): string {
    return this.view ? this.view.state.doc.toString() : "";
  }

  public getCurrentPath(): string | null {
    return this.currentPath;
  }

  public getIsDirty(): boolean {
    return this.isDirty;
  }

  public markClean(): void {
    this.isDirty = false;
    if (this.onDirtyChange) this.onDirtyChange(false);
  }
}
