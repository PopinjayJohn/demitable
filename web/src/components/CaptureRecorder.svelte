<script lang="ts">
  import { CONSENT_COPY, startCapture, uploadChunk } from "../lib/capture";
  import { appState } from "../lib/appstate.svelte";

  let consent = $state(false);
  let recording = $state(false);
  let status = $state("idle");
  let recorder: MediaRecorder | null = null;
  let chunkIndex = 0;
  const totalChunks = 4;

  const sid = $derived(appState.sessionId);

  async function toggleConsent() {
    consent = !consent;
  }

  async function start() {
    if (!consent) { status = "consent required"; return; }
    if (!sid) { status = "no session"; return; }
    try {
      recorder = await startCapture({
        sessionId: sid,
        onChunk: async (blob, idx) => {
          // total=0: the final chunk count is unknown until recording stops,
          // and the store assembles on demand from chunks (audio.Store.Read),
          // so we deliberately do not claim a total here.
          await uploadChunk(sid, blob, idx, 0);
        },
      });
      chunkIndex = 0;
      recorder.start(1000);
      recording = true;
      status = "recording";
      recorder.onstop = () => { recording = false; status = "stopped"; };
    } catch (e: any) {
      status = e?.message ?? "capture failed";
    }
  }

  async function stop() {
    if (recorder && recording) {
      recorder.stop();
      recording = false;
      status = "stopped";
    }
  }
</script>

<div class="capture">
  <label class="consent">
    <input type="checkbox" checked={consent} onchange={toggleConsent} />
    <span>{CONSENT_COPY}</span>
  </label>
  <div class="controls">
    <button onclick={start} disabled={recording || !consent}>Start recording</button>
    <button onclick={stop} disabled={!recording}>Stop</button>
    <span class="indicator" class:active={recording}>{recording ? "● RECORDING" : "idle"}</span>
  </div>
  <p class="status">{status}</p>
</div>

<style>
  .indicator.active { color: #c00; font-weight: bold; }
</style>
