/**
 * The prototype's shared state.
 *
 * A single store, because the point of the prototype is that the *same*
 * campaign renders differently for different principals — and the only way to
 * show that honestly is to have one source of truth and several projections
 * of it.
 */

import { NOTES, PRINCIPALS, VOCABULARY } from "./campaign";
import { project, type ProjectorOptions } from "./projection";
import type { Note, Principal, Projection } from "./model";
import { SHEET_FIELDS } from "./pack";

export type Surface = "dashboard" | "editor" | "character" | "player";
export type Proof = "none" | "greyscale" | "flat-hue" | "protanopia" | "deuteranopia" | "tritanopia";

export const projectorOptions: ProjectorOptions = { vocab: VOCABULARY, notes: NOTES };

class Store {
  surface = $state<Surface>("dashboard");
  /** The principal the whole app is currently rendering for. */
  principalId = $state<string>("gm");
  /** In the GM's own surfaces: render the note through a player's eyes. */
  playerEye = $state<boolean>(false);
  /** The player whose eyes the GM is borrowing, when `playerEye` is on. */
  playerEyeId = $state<string>("p_8f2a");
  selectedNoteId = $state<string>("01HQHOLLOW00000000000000");
  selectedCharacterId = $state<string>("01HQBRAM0000000000000000");
  proof = $state<Proof>("none");
  theme = $state<"light" | "dark">("light");
  /** The notes the GM has revealed during the walk-through. */
  revealed = $state<string[]>([]);
  showInspector = $state<boolean>(true);

  principal = $derived(PRINCIPALS.find((p) => p.id === this.principalId) ?? PRINCIPALS[0]);

  /** The principal a note is being rendered for, honouring the player eye. */
  effectivePrincipal = $derived(
    this.playerEye && this.principal.kind === "gm"
      ? (PRINCIPALS.find((p) => p.id === this.playerEyeId) ?? PRINCIPALS[0])
      : this.principal,
  );

  notes = $derived(NOTES);
  selectedNote = $derived(NOTES.find((n) => n.uid === this.selectedNoteId) ?? NOTES[0]);
  selectedCharacter = $derived(
    NOTES.find((n) => n.uid === this.selectedCharacterId) ?? NOTES[0],
  );

  /** The party members of a campaign. */
  party = $derived(
    PRINCIPALS.filter((p) => p.kind === "player" && p.campaign === "stormdark"),
  );

  projectionFor(note: Note, principal: Principal): Projection {
    return project(note, principal, projectorOptions);
  }

  /** The notes a principal may see. */
  visibleNotes(principal: Principal): { note: Note; projection: Projection }[] {
    return NOTES.filter((n) => {
      const p = this.projectionFor(n, principal);
      return p.visible;
    }).map((note) => ({ note, projection: this.projectionFor(note, principal) }));
  }

  /** The pack's sheet fields, resolved for a note and a principal. */
  sheetFor(note: Note, principal: Principal) {
    const projection = this.projectionFor(note, principal);
    const byKey = new Map(projection.properties.map((p) => [p.key, p]));
    return {
      projection,
      fields: SHEET_FIELDS.map((field) => {
        const prop = byKey.get(field.key);
        return {
          field,
          value: prop?.value,
          visible: prop !== undefined,
        };
      }),
    };
  }
}

export const store = new Store();
