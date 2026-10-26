import { EditorView, Decoration, WidgetType } from "@codemirror/view";
import type { DecorationSet } from "@codemirror/view";
import { StateField, RangeSetBuilder, type EditorState } from "@codemirror/state";
import type { Extension } from "@codemirror/state";

export interface RegisteredWidget {
  id: string;
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

function buildDecorations(state: EditorState, registry: WidgetRegistry): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>();
  const widgets = registry.all();
  let pos = 0;
  const doc = state.doc;
  while (pos < doc.length) {
    const line = doc.lineAt(pos);
    for (const w of widgets) {
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
    if (line.to >= doc.length) break;
    pos = line.to + 1;
  }
  return builder.finish();
}

export function widgetExtension(registry: WidgetRegistry): Extension {
  return StateField.define<DecorationSet>({
    create(state) {
      return buildDecorations(state, registry);
    },
    update(value, tr) {
      if (tr.docChanged) {
        return buildDecorations(tr.state, registry);
      }
      return value;
    },
    provide: (f) => EditorView.decorations.from(f),
  });
}
