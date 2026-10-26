import { EditorView, Decoration, ViewPlugin, ViewUpdate, WidgetType } from "@codemirror/view";
import type { DecorationSet } from "@codemirror/view";
import { RangeSetBuilder } from "@codemirror/state";
import type { Extension } from "@codemirror/state";

export interface RegisteredWidget {
  id: string;
  // A regex tested against a single source line. When it matches, the line
  // is replaced in the view by the rendered widget; the buffer is untouched.
  match: RegExp;
  render: (source: string) => HTMLElement;
}

export class WidgetRegistry {
  private items: RegisteredWidget[] = [];

  register(w: RegisteredWidget): void {
    this.items.push(w);
  }

  all(): RegisteredWidget[] {
    return this.items.slice();
  }

  clear(): void {
    this.items.length = 0;
  }
}

export const defaultWidgetRegistry = new WidgetRegistry();

class RenderedWidget extends WidgetType {
  constructor(private el: HTMLElement) {
    super();
  }
  toDOM(): HTMLElement {
    return this.el;
  }
  ignoreEvent(): boolean {
    return false;
  }
}

function buildDecorations(view: EditorView, registry: WidgetRegistry): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>();
  const widgets = registry.all();
  for (const { from, to } of view.visibleRanges) {
    let pos = from;
    while (pos <= to) {
      const line = view.state.doc.lineAt(pos);
      if (line.from > to) break;
      for (const w of widgets) {
        // reset lastIndex for global regexes
        w.match.lastIndex = 0;
        if (w.match.test(line.text)) {
          w.match.lastIndex = 0;
          const el = w.render(line.text);
          const deco = Decoration.replace({
            widget: new RenderedWidget(el),
            block: true,
          });
          builder.add(line.from, line.to, deco);
          break;
        }
      }
      if (line.to >= view.state.doc.length) break;
      pos = line.to + 1;
    }
  }
  return builder.finish();
}

export function widgetExtension(registry: WidgetRegistry): Extension {
  return ViewPlugin.fromClass(
    class {
      decorations: DecorationSet;
      constructor(view: EditorView) {
        this.decorations = buildDecorations(view, registry);
      }
      update(update: ViewUpdate) {
        if (update.docChanged || update.viewportChanged) {
          this.decorations = buildDecorations(update.view, registry);
        }
      }
    },
    { decorations: (v) => v.decorations }
  );
}
