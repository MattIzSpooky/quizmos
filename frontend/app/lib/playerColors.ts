// A small, curated set of cosmos-themed color choices for players to pick
// from when joining — deliberately not a full color picker, so every
// choice reads clearly against the dark theme and stays visually
// distinct from the app's own semantic colors (starlight/aurora/ember/
// flare). Must match the PlayerColor enum in api/openapi.yaml and
// api/asyncapi.yaml.
import type { components } from "./api/schema.gen";

export type PlayerColorId = components["schemas"]["PlayerColor"];

export interface PlayerColorOption {
  id: PlayerColorId;
  label: string;
  hex: string;
}

export const PLAYER_COLORS: PlayerColorOption[] = [
  { id: "nebula", label: "Nebula", hex: "#f472b6" },
  { id: "comet", label: "Comet", hex: "#60a5fa" },
  { id: "nova", label: "Nova", hex: "#a78bfa" },
  { id: "quasar", label: "Quasar", hex: "#22d3ee" },
  { id: "solar", label: "Solar", hex: "#fbbf24" },
  { id: "crimson", label: "Crimson", hex: "#ef4444" },
];

export const DEFAULT_PLAYER_COLOR = PLAYER_COLORS[0].id;

export function playerColorHex(id: string | undefined): string {
  return PLAYER_COLORS.find((c) => c.id === id)?.hex ?? PLAYER_COLORS[0].hex;
}
