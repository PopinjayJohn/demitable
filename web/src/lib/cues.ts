// Local music cues for P4. Cues are served through the EXISTING
// role-scoped asset path (/asset/...) — no new serving mechanism, no
// networked dependency, no fetch in the playback path.

export interface Cue {
  mood: string;
  src: string; // local asset path, e.g. /asset/music/tense.mp3
  title?: string; // GM-chosen text; passage-adjacent for display
  id?: string;
}

export interface DisplayCue {
  mood: string;
  src: string;
  track: string; // non-GM-chosen label (basename only)
}

export function isLocalCueSrc(src: unknown): boolean {
  const s = String(src ?? "").trim();
  if (s === "") return false;
  if (/^https?:\/\//i.test(s)) return false;
  // Anything that looks like a protocol/remote URL is rejected.
  if (/^[a-z]+:/i.test(s) && !s.startsWith("/")) return false;
  return true;
}

export function cueBasename(src: string): string {
  const s = String(src ?? "").trim();
  const parts = s.split(/[\\/]/);
  return parts[parts.length - 1] || s;
}

export function cuesForMood(cues: Cue[], mood: string): Cue[] {
  return (cues ?? []).filter((c) => c && String(c.mood).toLowerCase() === String(mood).toLowerCase() && isLocalCueSrc(c.src));
}

export function selectCue(cues: Cue[], mood: string, index = 0): Cue | null {
  const list = cuesForMood(cues, mood);
  if (list.length === 0) return null;
  const i = Math.max(0, Math.min(index, list.length - 1));
  return list[i];
}

export function projectCueForDisplay(cue: Cue | null | undefined): DisplayCue | null {
  if (!cue || !isLocalCueSrc(cue.src)) return null;
  return {
    mood: String(cue.mood ?? ""),
    src: String(cue.src),
    track: cueBasename(String(cue.src)),
  };
}

export function cueSelectableBy(principalKind: string, cue: Cue | null | undefined): boolean {
  // Only GM principals select cues. Forged seats (seat/*) are rejected.
  if (!cue) return false;
  return principalKind === "gm";
}

export function selectCueGuarded(principalKind: string, cues: Cue[], mood: string, index = 0): Cue {
  if (principalKind !== "gm") {
    throw new Error("forbidden: cue selection requires GM");
  }
  const cue = selectCue(cues, mood, index);
  if (!cue) throw new Error("no cue for mood");
  return cue;
}

// Playback controller. Uses the platform Audio element only; never fetches.
export interface PlaybackHandle {
  audio: HTMLAudioElement | null;
  current: Cue | null;
  play(cue: Cue): void;
  stop(): void;
}

export function createPlayback(getAudio?: () => HTMLAudioElement): PlaybackHandle {
  let audio: HTMLAudioElement | null = null;
  try {
    audio = getAudio ? getAudio() : (typeof Audio !== "undefined" ? new Audio() : null);
  } catch {
    audio = null;
  }
  const handle: PlaybackHandle = {
    audio,
    current: null,
    play(cue: Cue) {
      if (!isLocalCueSrc(cue?.src)) return;
      if (!audio) return;
      handle.current = cue;
      audio.src = String(cue.src);
      // no fetch: setting src on a media element lets the browser load it,
      // but our network-down test asserts no `fetch` is invoked.
      try { void audio.play(); } catch { /* jsdom: ignore */ }
    },
    stop() {
      if (audio) {
        try { audio.pause(); } catch { /* ignore */ }
        audio.removeAttribute("src");
      }
      handle.current = null;
    },
  };
  return handle;
}
