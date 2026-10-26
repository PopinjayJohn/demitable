/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from "vitest";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { WidgetRegistry, widgetExtension } from "./widgets";

describe("widget registry", () => {
  it("registers a third-party widget without mutating source", () => {
    const reg = new WidgetRegistry();
    reg.register({
      id: "third-party",
      match: /^widget:/,
      render: (src) => {
        const el = document.createElement("div");
        el.textContent = src;
        return el;
      },
    });
    expect(reg.all().length).toBe(1);
    expect(reg.all()[0].id).toBe("third-party");
  });

  it("decoration leaves the buffer byte-identical", () => {
    const reg = new WidgetRegistry();
    reg.register({
      id: "slot",
      match: /^slot:/,
      render: (src) => {
        const el = document.createElement("span");
        el.textContent = src;
        return el;
      },
    });
    const doc = "slot: 3\nplain text\n";
    const state = EditorState.create({
      doc,
      extensions: [widgetExtension(reg)],
    });
    const parent = document.createElement("div");
    document.body.appendChild(parent);
    const view = new EditorView({ state, parent });
    // decorations must not change the document
    expect(view.state.doc.toString()).toBe(doc);
    view.destroy();
  });
});
