import { useEffect, useRef, useState } from "react";

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function PlayIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className="h-3.5 w-3.5 translate-x-0.5" aria-hidden="true">
      <path d="M7 5.5v13l11-6.5-11-6.5Z" />
    </svg>
  );
}

function PauseIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className="h-3.5 w-3.5" aria-hidden="true">
      <rect x="6" y="5" width="4" height="14" rx="1" />
      <rect x="14" y="5" width="4" height="14" rx="1" />
    </svg>
  );
}

// Fully hand-style both the track and the thumb rather than leaning on
// the browser/OS default plus `accent-color` for a tint: iOS Safari in
// particular keeps rendering its own native track fill even once the
// thumb is restyled unless `::-webkit-slider-runnable-track` is
// overridden too, which is what made the bar look inconsistent on an
// iPhone. Setting every pseudo-element explicitly (track height/color,
// thumb size/color, and a webkit-only vertical offset to center the
// thumb on the thinner track) makes it render identically everywhere.
const RANGE_INPUT_CLASSES = [
  "appearance-none bg-transparent cursor-pointer",
  "[&::-webkit-slider-runnable-track]:h-1.5 [&::-webkit-slider-runnable-track]:rounded-full [&::-webkit-slider-runnable-track]:bg-void-3",
  "[&::-moz-range-track]:h-1.5 [&::-moz-range-track]:rounded-full [&::-moz-range-track]:bg-void-3",
  "[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:mt-[-3px] [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-starlight [&::-webkit-slider-thumb]:cursor-pointer",
  "[&::-moz-range-thumb]:h-3 [&::-moz-range-thumb]:w-3 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-starlight [&::-moz-range-thumb]:cursor-pointer",
].join(" ");

function VolumeIcon({ muted }: { muted: boolean }) {
  return (
    <svg viewBox="0 0 24 24" className="h-4 w-4" aria-hidden="true">
      <path d="M11 5 6 9H3v6h3l5 4V5Z" fill="currentColor" />
      {muted ? (
        <path
          d="m16 9 4.5 6M20.5 9 16 15"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        />
      ) : (
        <path
          d="M15.5 8.5a5 5 0 0 1 0 7M18 6a9 9 0 0 1 0 12"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        />
      )}
    </svg>
  );
}

/**
 * A themed stand-in for the browser's native `<audio controls>` bar, whose
 * platform chrome (light, OS-styled) doesn't fit the app's dark cosmos
 * theme. Used everywhere a question's audio fragment plays: the player
 * screen, the admin live-question preview, and the quiz editor. Never
 * autoplays — playback only ever starts from a press on the play button.
 */
export function AudioPlayer({ src, className = "" }: { src: string; className?: string }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [duration, setDuration] = useState(0);
  const [currentTime, setCurrentTime] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);

  // A new src (different question, or media replaced on the same one)
  // always starts paused from the top rather than inheriting whatever
  // state the previous clip was left in. Volume/mute deliberately persist
  // across clips — someone who turned it down doesn't want it back at
  // full blast for the next question.
  useEffect(() => {
    setPlaying(false);
    setCurrentTime(0);
    setDuration(0);
  }, [src]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    audio.volume = volume;
    audio.muted = muted;
  }, [volume, muted]);

  function togglePlay() {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      audio.play();
    } else {
      audio.pause();
    }
  }

  function seek(e: React.ChangeEvent<HTMLInputElement>) {
    const time = Number(e.target.value);
    setCurrentTime(time);
    if (audioRef.current) audioRef.current.currentTime = time;
  }

  function changeVolume(e: React.ChangeEvent<HTMLInputElement>) {
    const v = Number(e.target.value);
    setVolume(v);
    if (v > 0) setMuted(false);
  }

  const effectiveMuted = muted || volume === 0;

  return (
    <div
      className={`flex items-center gap-3 rounded-xl border border-void-3 bg-void-2/80 px-4 py-2.5 ${className}`}
    >
      <audio
        ref={audioRef}
        src={src}
        preload="metadata"
        className="hidden"
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
        onLoadedMetadata={(e) => setDuration(e.currentTarget.duration)}
        onTimeUpdate={(e) => setCurrentTime(e.currentTarget.currentTime)}
      />
      <button
        type="button"
        onClick={togglePlay}
        aria-label={playing ? "Pause" : "Play"}
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-starlight text-void transition hover:brightness-110"
      >
        {playing ? <PauseIcon /> : <PlayIcon />}
      </button>
      <input
        type="range"
        min={0}
        max={duration || 0}
        step={0.01}
        value={currentTime}
        onChange={seek}
        aria-label="Seek"
        // min-w-0 overrides the flex item default of min-width: auto —
        // without it, some browsers (Safari in particular) refuse to
        // shrink a <input type="range"> below its native intrinsic
        // width, so on a narrow phone the row's total content (play
        // button + this + the time readout + mute button) overflows
        // past the card's right edge instead of the bar actually
        // shrinking to fit.
        className={`h-3 min-w-0 flex-1 ${RANGE_INPUT_CLASSES}`}
      />
      <span className="shrink-0 font-mono text-xs tabular-nums text-dim">
        {formatTime(currentTime)} / {formatTime(duration)}
      </span>
      <button
        type="button"
        onClick={() => setMuted((m) => !m)}
        aria-label={effectiveMuted ? "Unmute" : "Mute"}
        className="flex shrink-0 items-center justify-center text-dim transition hover:text-paper"
      >
        <VolumeIcon muted={effectiveMuted} />
      </button>
      <input
        type="range"
        min={0}
        max={1}
        step={0.01}
        value={effectiveMuted ? 0 : volume}
        onChange={changeVolume}
        aria-label="Volume"
        className={`hidden h-3 w-16 shrink-0 sm:block ${RANGE_INPUT_CLASSES}`}
      />
    </div>
  );
}
