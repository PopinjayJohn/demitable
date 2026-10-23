/**
 * The icon set.
 *
 * Inline SVG, no icon font, no network. Two reasons:
 *
 *  - **Shape, not hue.** ADR-0034 says no state may be encoded in colour
 *    alone. Every state in this app has a glyph, and the glyphs are chosen to
 *    be distinguishable *by outline* — a lock is a lock in greyscale.
 *  - **No font, no network, no FOUT.** The prototype has to render the same
 *    in a headless browser as in a browser with a network.
 *
 * Every glyph is drawn on a 24×24 grid with a 2px stroke, so they line up.
 */

export type IconName =
  | "lock"
  | "users"
  | "globe"
  | "user"
  | "eye-off"
  | "eye"
  | "chevron-down"
  | "chevron-right"
  | "quote"
  | "list"
  | "note"
  | "example"
  | "tip"
  | "warning"
  | "info"
  | "pencil"
  | "dice"
  | "file"
  | "scroll"
  | "shield"
  | "sparkle"
  | "grip";

const PATHS: Record<IconName, string> = {
  // A padlock: a shackle and a body.
  lock: "M7 10V7a5 5 0 0 1 10 0v3M5 10h14v10H5zM12 14v3",
  // Two figures.
  users: "M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3 20a6 6 0 0 1 12 0M17 11a2.5 2.5 0 1 0 0-5M16 20h5a5 5 0 0 0-3.5-4.8",
  // A globe: a circle with a meridian.
  globe: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3.5 9h17M3.5 15h17M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z",
  // One figure.
  user: "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0",
  // A closed eye.
  "eye-off": "M3 3l18 18M10.6 10.6a3 3 0 0 0 4.2 4.2M9.4 5.2A9.5 9.5 0 0 1 12 5c5 0 9 4.5 9 7a12 12 0 0 1-2.4 3.4M6.2 6.7A12 12 0 0 0 3 12c0 2.5 4 7 9 7a9.7 9.7 0 0 0 3.4-.6",
  // An open eye.
  eye: "M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7-10-7-10-7zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
  "chevron-down": "M6 9l6 6 6-6",
  "chevron-right": "M9 6l6 6-6 6",
  // A pair of quote marks.
  quote: "M9 7c-2.2 0-4 1.8-4 4v6h6v-6H7c0-1.1.9-2 2-2V7zM19 7c-2.2 0-4 1.8-4 4v6h6v-6h-4c0-1.1.9-2 2-2V7z",
  // Three lines with a lead.
  list: "M9 6h12M9 12h12M9 18h12M4 6h1M4 12h1M4 18h1",
  // A pen.
  note: "M4 20h4L20 8l-4-4L4 16v4zM14 6l4 4",
  // A sheet with a folded corner.
  example: "M6 3h8l4 4v14H6zM14 3v4h4M9 12h6M9 16h6",
  // A bulb.
  tip: "M9 18h6M10 21h4M12 3a6 6 0 0 0-3.5 10.9c.6.5.9 1.2 1 2h5c.1-.8.4-1.5 1-2A6 6 0 0 0 12 3z",
  // A triangle with a bar.
  warning: "M12 4l9 16H3zM12 10v4M12 17h.01",
  // A circle with a bar.
  info: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v6M12 8h.01",
  pencil: "M4 20h4L20 8l-4-4L4 16v4zM14 6l4 4",
  // A die.
  dice: "M5 3h14v18H5zM8.5 7.5h.01M15.5 7.5h.01M12 12h.01M8.5 16.5h.01M15.5 16.5h.01",
  file: "M6 3h8l4 4v14H6zM14 3v4h4",
  scroll: "M6 4h11a2 2 0 0 1 2 2v12a2 2 0 0 0 2 2H8a2 2 0 0 1-2-2zM9 8h7M9 12h7M9 16h4",
  shield: "M12 3l8 3v6c0 4.5-3.2 8.2-8 9-4.8-.8-8-4.5-8-9V6z",
  sparkle: "M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8zM19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z",
  grip: "M9 6h.01M15 6h.01M9 12h.01M15 12h.01M9 18h.01M15 18h.01",
};

export const ICON_PATHS = PATHS;
