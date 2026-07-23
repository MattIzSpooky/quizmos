import { useEffect, useState } from "react";

interface OrbitTimerProps {
  /** Total time for one lap, in seconds. */
  totalSeconds: number;
  /** Remounts (via a changing `key` from the caller) reset the lap. */
  size?: number;
}

/**
 * A countdown rendered as one orbit: a body sweeps a full lap of the ring
 * over totalSeconds, instead of a generic linear progress bar. The
 * remaining whole-second count sits at the center as the numeric fallback
 * (and the only thing screen readers / reduced-motion users need).
 */
export function OrbitTimer({ totalSeconds, size = 96 }: OrbitTimerProps) {
  const [remaining, setRemaining] = useState(totalSeconds);

  useEffect(() => {
    setRemaining(totalSeconds);
    const startedAt = Date.now();
    const interval = setInterval(() => {
      const elapsed = (Date.now() - startedAt) / 1000;
      setRemaining(Math.max(0, totalSeconds - elapsed));
    }, 100);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [totalSeconds]);

  const fraction = totalSeconds > 0 ? remaining / totalSeconds : 0;
  const radius = size / 2 - 8;
  const circumference = 2 * Math.PI * radius;
  const angle = (1 - fraction) * 360 - 90;
  const bodyX = size / 2 + radius * Math.cos((angle * Math.PI) / 180);
  const bodyY = size / 2 + radius * Math.sin((angle * Math.PI) / 180);
  const urgent = remaining <= 5 && remaining > 0;

  return (
    <div
      className="relative shrink-0"
      style={{ width: size, height: size }}
      role="timer"
      aria-live="polite"
      aria-label={`${Math.ceil(remaining)} seconds remaining`}
    >
      <svg width={size} height={size} className="-rotate-0">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--color-void-3)"
          strokeWidth={2}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={urgent ? "var(--color-flare)" : "var(--color-starlight)"}
          strokeWidth={2}
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - fraction)}
          strokeLinecap="round"
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
          className="transition-[stroke-dashoffset] duration-100 ease-linear motion-reduce:transition-none"
        />
        <circle
          cx={bodyX}
          cy={bodyY}
          r={4}
          fill={urgent ? "var(--color-flare)" : "var(--color-starlight)"}
          className="motion-reduce:hidden"
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center">
        <span
          className={`font-mono text-lg tabular-nums ${urgent ? "text-flare" : "text-paper"}`}
        >
          {Math.ceil(remaining)}
        </span>
      </div>
    </div>
  );
}
