// A quiet, fixed backdrop shared by every route: a sparse field of stars
// plus two soft glow blooms. Star positions are generated with a seeded
// PRNG (not Math.random) so server and client render identically and
// hydration never mismatches.
function mulberry32(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

interface Star {
  x: number;
  y: number;
  size: number;
  opacity: number;
  delay: number;
  duration: number;
}

function makeStars(count: number, seed: number): Star[] {
  const rand = mulberry32(seed);
  return Array.from({ length: count }, () => ({
    x: rand() * 100,
    y: rand() * 100,
    size: 1 + rand() * 1.6,
    opacity: 0.25 + rand() * 0.55,
    delay: rand() * 6,
    duration: 3 + rand() * 4,
  }));
}

const STARS = makeStars(90, 1337);

export function Starfield() {
  return (
    <div aria-hidden="true" className="pointer-events-none fixed inset-0 -z-10 overflow-hidden bg-void">
      <div
        className="absolute -left-24 -top-32 h-[28rem] w-[28rem] rounded-full opacity-20 blur-[100px]"
        style={{ background: "var(--color-starlight)" }}
      />
      <div
        className="absolute -bottom-40 -right-16 h-[26rem] w-[26rem] rounded-full opacity-[0.15] blur-[110px]"
        style={{ background: "var(--color-aurora)" }}
      />
      {STARS.map((star, i) => (
        <span
          key={i}
          className="absolute rounded-full bg-paper motion-safe:animate-[twinkle_var(--dur)_ease-in-out_infinite]"
          style={
            {
              left: `${star.x}%`,
              top: `${star.y}%`,
              width: `${star.size}px`,
              height: `${star.size}px`,
              opacity: star.opacity,
              animationDelay: `${star.delay}s`,
              "--dur": `${star.duration}s`,
              "--twinkle-min": star.opacity * 0.4,
            } as React.CSSProperties
          }
        />
      ))}
    </div>
  );
}
