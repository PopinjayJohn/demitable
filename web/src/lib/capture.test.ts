import { describe, it, expect } from "vitest";
import { CONSENT_COPY } from "./capture";

describe("capture consent copy", () => {
  it("is pinned verbatim", () => {
    expect(CONSENT_COPY).toBe(
      "Audio is recorded. Enabling capture records audio of the table and stores it on this machine; obtain the table's consent first."
    );
  });
  it("states plainly that audio is recorded", () => {
    expect(CONSENT_COPY.toLowerCase()).toContain("audio is recorded");
  });
});
