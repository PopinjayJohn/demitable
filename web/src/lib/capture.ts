// Capture helpers for P1: browser recording via getUserMedia + MediaRecorder.
// Consent copy is pinned here; tests assert the verbatim string.

export const CONSENT_COPY =
  "Audio is recorded. Enabling capture records audio of the table and stores it on this machine; obtain the table's consent first.";

export interface CaptureOptions {
  sessionId: string;
  onChunk?: (blob: Blob, index: number) => void | Promise<void>;
  onProgress?: (bytes: number) => void;
}

export async function startCapture(opts: CaptureOptions): Promise<MediaRecorder> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const rec = new MediaRecorder(stream);
  let index = 0;
  rec.ondataavailable = async (e) => {
    if (e.data && e.data.size > 0) {
      if (opts.onChunk) await opts.onChunk(e.data, index++);
      if (opts.onProgress) opts.onProgress(e.data.size);
    }
  };
  return rec;
}

export async function uploadChunk(sessionId: string, blob: Blob, index: number, total: number): Promise<Response> {
  const { apiFetch } = await import("./auth");
  return apiFetch(`/api/v1/sessions/${encodeURIComponent(sessionId)}/audio?chunk=${index}&total=${total}`, {
    method: "POST",
    headers: { "Content-Type": blob.type || "application/octet-stream" },
    body: blob,
  });
}
