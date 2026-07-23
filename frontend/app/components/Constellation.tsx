export interface ConstellationEntry {
  clientId: string;
  nickname: string;
  score: number;
  rank: number;
  /** Omit when connection state isn't tracked (e.g. the player-facing view). */
  connected?: boolean;
}

/**
 * The leaderboard, rendered as a constellation: each player is a star
 * sized and lit by their score (its "magnitude"), joined in rank order by
 * a faint line — the same shape a star chart uses to turn a list of
 * points into a recognizable figure.
 */
export function Constellation({ entries }: { entries: ConstellationEntry[] }) {
  if (entries.length === 0) {
    return (
      <p className="rounded-lg border border-void-3 bg-void-2 px-4 py-6 text-center text-sm text-dim">
        No signal yet — the constellation forms once scores come in.
      </p>
    );
  }

  const maxScore = Math.max(...entries.map((e) => e.score), 1);

  return (
    <ol className="flex flex-col gap-4">
      {entries.map((entry, i) => {
        const magnitude = entry.score / maxScore;
        const diameter = 7 + magnitude * 11;
        const isBrightest = i === 0;
        return (
          <li
            key={entry.clientId}
            className="relative flex items-center gap-3 motion-safe:animate-[star-in_0.5s_ease-out_both]"
            style={{ animationDelay: `${i * 60}ms` }}
          >
            <div className="relative flex w-8 shrink-0 items-center justify-center self-stretch">
              <span
                aria-hidden="true"
                className="rounded-full"
                style={{
                  width: diameter,
                  height: diameter,
                  background: isBrightest ? "var(--color-starlight)" : "var(--color-paper)",
                  boxShadow: isBrightest
                    ? "0 0 14px 2px color-mix(in srgb, var(--color-starlight) 70%, transparent)"
                    : "0 0 6px 0 color-mix(in srgb, var(--color-paper) 40%, transparent)",
                  outline:
                    entry.connected === false
                      ? "1px solid var(--color-void-3)"
                      : entry.connected === true
                        ? "1px solid color-mix(in srgb, var(--color-aurora) 70%, transparent)"
                        : "none",
                  outlineOffset: 3,
                }}
              />
              {i < entries.length - 1 && (
                <span
                  aria-hidden="true"
                  className="absolute left-1/2 top-full h-4 w-px -translate-x-1/2 bg-void-3"
                />
              )}
            </div>
            <span
              className={`min-w-0 flex-1 truncate font-display text-base ${
                entry.connected === false ? "text-dim" : "text-paper"
              }`}
            >
              {entry.nickname}
            </span>
            <span className="shrink-0 font-mono text-sm tabular-nums text-starlight-dim">
              {entry.score}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
