import { describe, it, expect } from "vitest";
import { campaignOf, campaignMatches, filterByCampaign } from "./campaign";

describe("campaignOf", () => {
  it("defaults missing/empty campaign to 'No campaign'", () => {
    expect(campaignOf({})).toBe("No campaign");
    expect(campaignOf({ campaign: "" })).toBe("No campaign");
    expect(campaignOf({ campaign: null })).toBe("No campaign");
    expect(campaignOf({ campaign: "stormdark" })).toBe("stormdark");
  });
});

describe("campaignMatches (filter predicate)", () => {
  it("null filter matches everything", () => {
    expect(campaignMatches({ campaign: "a" }, null)).toBe(true);
    expect(campaignMatches({ campaign: "" }, null)).toBe(true);
  });

  it("undefined/empty filter matches everything", () => {
    expect(campaignMatches({ campaign: "a" }, undefined)).toBe(true);
    expect(campaignMatches({ campaign: "a" }, "")).toBe(true);
  });

  it("matches by exact campaign name", () => {
    expect(campaignMatches({ campaign: "stormdark" }, "stormdark")).toBe(true);
    expect(campaignMatches({ campaign: "stormdark" }, "other")).toBe(false);
  });

  it("treats missing campaign as 'No campaign'", () => {
    expect(campaignMatches({}, "No campaign")).toBe(true);
    expect(campaignMatches({ campaign: "" }, "No campaign")).toBe(true);
    expect(campaignMatches({}, "stormdark")).toBe(false);
  });
});

describe("filterByCampaign", () => {
  it("returns all notes when filter is null", () => {
    const notes = [{ campaign: "a" }, { campaign: "b" }] as any;
    expect(filterByCampaign(notes, null)).toHaveLength(2);
  });

  it("filters to matching campaign", () => {
    const notes = [{ campaign: "a" }, { campaign: "b" }, { campaign: "" }] as any;
    expect(filterByCampaign(notes, "a")).toHaveLength(1);
    expect(filterByCampaign(notes, "No campaign")).toHaveLength(1);
  });
});
