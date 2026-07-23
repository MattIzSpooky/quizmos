import { useState } from "react";
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";

export function Panel({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-2xl border border-void-3 bg-void-2/80 backdrop-blur-sm ${className}`}
    >
      {children}
    </div>
  );
}

type ButtonVariant = "primary" | "ghost" | "danger";

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    "bg-starlight text-void hover:brightness-110 disabled:brightness-75 disabled:opacity-50",
  ghost:
    "border border-void-3 text-paper hover:border-starlight-dim disabled:opacity-40",
  danger:
    "border border-flare/40 text-flare hover:border-flare hover:bg-flare/10 disabled:opacity-40",
};

export function Button({
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-2 rounded-full px-5 py-3 font-display text-sm font-semibold tracking-wide transition disabled:cursor-not-allowed ${buttonVariants[variant]} ${className}`}
      {...props}
    />
  );
}

export function Field({
  label,
  className = "",
  ...props
}: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-semibold uppercase tracking-[0.2em] text-dim">
        {label}
      </span>
      <input
        className={`rounded-lg border border-void-3 bg-void px-4 py-3 text-paper placeholder-dim/60 outline-none transition focus:border-aurora ${className}`}
        {...props}
      />
    </label>
  );
}

export function Toggle({
  checked,
  onChange,
  label,
  description,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  description?: string;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-4">
      <span>
        <span className="block text-sm text-paper">{label}</span>
        {description && <span className="block text-xs text-dim">{description}</span>}
      </span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={`flex h-6 w-11 shrink-0 items-center rounded-full p-0.5 transition ${
          checked ? "bg-starlight" : "bg-void-3"
        }`}
      >
        <span
          aria-hidden="true"
          className={`h-5 w-5 rounded-full bg-void transition-transform ${
            checked ? "translate-x-5" : "translate-x-0"
          }`}
        />
      </button>
    </label>
  );
}

/**
 * Click-to-rename text: a plain label until clicked, then an input that
 * saves on blur/Enter and discards the edit on Escape. Used for a quiz's
 * title and a question's prompt — anywhere a short bit of text is the
 * thing being renamed rather than a whole form field with its own label.
 */
export function InlineEditableText({
  value,
  onSave,
  ariaLabel,
  className = "",
  inputClassName = "",
}: {
  value: string;
  onSave: (next: string) => void;
  ariaLabel: string;
  className?: string;
  inputClassName?: string;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);

  function startEditing() {
    setDraft(value);
    setEditing(true);
  }

  function commit() {
    setEditing(false);
    const trimmed = draft.trim();
    if (trimmed && trimmed !== value) onSave(trimmed);
  }

  if (editing) {
    return (
      <input
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            e.currentTarget.blur();
          } else if (e.key === "Escape") {
            e.preventDefault();
            setEditing(false);
          }
        }}
        aria-label={ariaLabel}
        className={`rounded-lg border border-aurora bg-void px-2 py-1 outline-none ${inputClassName}`}
      />
    );
  }

  return (
    <button
      type="button"
      onClick={startEditing}
      title="Click to rename"
      className={`rounded-lg px-1 py-0.5 text-left transition hover:bg-void-3/60 ${className}`}
    >
      {value}
    </button>
  );
}

/** Marks which option is correct — the "true star" of the question. */
export function StarToggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      onClick={onChange}
      title={checked ? "Correct answer" : "Mark as the correct answer"}
      className="flex shrink-0 items-center justify-center rounded-full p-1 text-lg leading-none transition hover:scale-110"
    >
      <span className={checked ? "text-starlight" : "text-void-3"} aria-hidden="true">
        {checked ? "★" : "☆"}
      </span>
      <span className="sr-only">{label}</span>
    </button>
  );
}
