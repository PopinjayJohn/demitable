// Shared campaign-filter predicate.
//
// The sidebar's CAMPAIGNS list and the vault explorer both narrow the
// note set by the active campaign. The predicate is deliberately a single
// function so the two surfaces can never drift into different wordings:
// `null` (or any falsy value) means "all campaigns", otherwise a note
// matches when its campaign (defaulting to "No campaign") equals the
// filter exactly.
import type { NoteSummary } from "./api";

export type CampaignFilter = string | null | undefined;

export function campaignOf(note: unknown): string {
  return String((note as any)?.campaign || "No campaign");
}

export function campaignMatches(note: unknown, filter: CampaignFilter): boolean {
  if (!filter) return true;
  return campaignOf(note) === filter;
}

// Array form, for callers that want the filtered set directly.
export function filterByCampaign<T = NoteSummary>(notes: readonly T[], filter: CampaignFilter): T[] {
  return (notes ?? []).filter((n) => campaignMatches(n, filter));
}
