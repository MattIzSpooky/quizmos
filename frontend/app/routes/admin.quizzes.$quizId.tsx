import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/admin.quizzes.$quizId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";
import { AdminHeader } from "../components/AdminHeader";
import { AudioPlayer } from "../components/AudioPlayer";
import { Button, Field, InlineEditableText, Panel, StarToggle, Toggle } from "../components/ui";

type QuizDetail = components["schemas"]["QuizDetail"];
type QuestionType = components["schemas"]["QuestionType"];

const EMPTY_OPTIONS = [
  { text: "", isCorrect: true },
  { text: "", isCorrect: false },
];

const MAX_FREE_TEXT_ANSWER_LENGTH = 500;

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Edit quiz" }];
}

export default function AdminQuizDetail({ params }: Route.ComponentProps) {
  const ready = useRequireAdmin();
  const navigate = useNavigate();
  const quizId = params.quizId;
  const [quiz, setQuiz] = useState<QuizDetail | null>(null);
  const [questionType, setQuestionType] = useState<QuestionType>("multiple_choice");
  const [prompt, setPrompt] = useState("");
  const [options, setOptions] = useState(EMPTY_OPTIONS);
  // Drag-to-reorder state: which question is being dragged, and which
  // one it's currently hovering over (for the drop-position indicator).
  // Neither touches `quiz` directly — the list only actually reorders
  // once the drop lands and the server confirms the new order.
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);

  async function load() {
    const { data } = await adminApi.GET("/admin/quizzes/{quizId}", { params: { path: { quizId } } });
    if (data) setQuiz(data);
  }

  useEffect(() => {
    if (ready) load();
  }, [ready]);

  function updateOptionText(i: number, text: string) {
    setOptions((prev) => prev.map((o, idx) => (idx === i ? { ...o, text } : o)));
  }

  function setCorrect(i: number) {
    setOptions((prev) => prev.map((o, idx) => ({ ...o, isCorrect: idx === i })));
  }

  function addOption() {
    setOptions((prev) => [...prev, { text: "", isCorrect: false }]);
  }

  async function addQuestion(e: React.FormEvent) {
    e.preventDefault();
    if (!prompt.trim()) return;
    if (questionType === "multiple_choice" && options.some((o) => !o.text.trim())) return;
    await adminApi.POST("/admin/quizzes/{quizId}/questions", {
      params: { path: { quizId } },
      body: {
        type: questionType,
        prompt: prompt.trim(),
        timeLimitSeconds: 30,
        points: 1000,
        options: questionType === "multiple_choice" ? options : [],
      },
    });
    setPrompt("");
    setOptions(EMPTY_OPTIONS);
    load();
  }

  async function deleteQuestion(questionId: string) {
    await adminApi.DELETE("/admin/quizzes/{quizId}/questions/{questionId}", {
      params: { path: { quizId, questionId } },
    });
    load();
  }

  async function moveQuestionBefore(draggedId: string, targetId: string) {
    if (!quiz || draggedId === targetId) return;
    const ids = quiz.questions.map((q) => q.id);
    const from = ids.indexOf(draggedId);
    if (from === -1) return;
    ids.splice(from, 1);
    const to = ids.indexOf(targetId);
    ids.splice(to === -1 ? ids.length : to, 0, draggedId);

    await adminApi.PUT("/admin/quizzes/{quizId}/questions/order", {
      params: { path: { quizId } },
      body: { questionIds: ids },
    });
    load();
  }

  async function uploadMedia(questionId: string, file: File) {
    const formData = new FormData();
    formData.append("file", file);
    await adminApi.POST("/admin/quizzes/{quizId}/questions/{questionId}/media", {
      params: {
        path: { quizId, questionId },
        // The real token is injected by adminApi's auth middleware for
        // every request on this client — this placeholder only exists to
        // satisfy the header param's type (see api/openapi.yaml: this
        // operation checks auth itself rather than via the usual
        // security scheme, since standard request validation would
        // otherwise consume the multipart body before it reaches MinIO).
        header: { Authorization: "" },
      },
      // openapi-typescript types a multipart requestBody by its schema
      // shape ({ file: string }), not as FormData — but FormData is
      // exactly what a browser multipart upload has to send.
      body: formData as unknown as { file: string },
    });
    load();
  }

  async function removeMedia(questionId: string) {
    await adminApi.DELETE("/admin/quizzes/{quizId}/questions/{questionId}/media", {
      params: { path: { quizId, questionId }, header: { Authorization: "" } },
    });
    load();
  }

  async function startGame() {
    const { data } = await adminApi.POST("/admin/games", { body: { quizId } });
    if (data) navigate(`/admin/games/${data.id}`);
  }

  async function setTimed(timed: boolean) {
    setQuiz((prev) => (prev ? { ...prev, timed } : prev));
    await adminApi.PATCH("/admin/quizzes/{quizId}", { params: { path: { quizId } }, body: { timed } });
  }

  async function renameQuiz(title: string) {
    setQuiz((prev) => (prev ? { ...prev, title } : prev));
    await adminApi.PATCH("/admin/quizzes/{quizId}", { params: { path: { quizId } }, body: { title } });
  }

  async function renameQuestion(questionId: string, prompt: string) {
    setQuiz((prev) =>
      prev
        ? { ...prev, questions: prev.questions.map((q) => (q.id === questionId ? { ...q, prompt } : q)) }
        : prev
    );
    await adminApi.PATCH("/admin/quizzes/{quizId}/questions/{questionId}", {
      params: { path: { quizId, questionId } },
      body: { prompt },
    });
  }

  if (!ready || !quiz) return null;

  return (
    <main className="relative z-0 mx-auto max-w-2xl px-4 py-8 sm:py-12">
      <AdminHeader back={{ to: "/admin/quizzes", label: "All quizzes" }} />

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="min-w-0 flex-1">
          <InlineEditableText
            value={quiz.title}
            onSave={renameQuiz}
            ariaLabel="Quiz title"
            className="block w-full truncate font-display text-2xl font-semibold text-paper"
            inputClassName="w-full font-display text-2xl font-semibold"
          />
        </h1>
        <Button onClick={startGame} disabled={quiz.questions.length === 0} className="shrink-0">
          Start a new game
        </Button>
      </div>

      <Panel className="mt-6 p-5">
        <Toggle
          checked={quiz.timed}
          onChange={setTimed}
          label="Timed questions"
          description="Off for a slower, no-pressure pace — no countdown shown to players."
        />
      </Panel>

      <section className="mt-8 flex flex-col gap-3">
        <h2 className="font-mono text-xs uppercase tracking-[0.2em] text-dim">Questions</h2>
        {quiz.questions.length === 0 && (
          <p className="text-sm text-dim">No questions yet — add one below.</p>
        )}
        <ul className="flex flex-col gap-3">
          {quiz.questions.map((q) => (
            <li
              key={q.id}
              onDragOver={(e) => {
                if (!draggingId) return;
                e.preventDefault();
                e.dataTransfer.dropEffect = "move";
                setDragOverId(q.id);
              }}
              onDrop={(e) => {
                e.preventDefault();
                if (draggingId) moveQuestionBefore(draggingId, q.id);
                setDraggingId(null);
                setDragOverId(null);
              }}
              className={`rounded-2xl transition ${
                dragOverId === q.id && draggingId !== q.id ? "outline outline-2 outline-aurora" : ""
              } ${draggingId === q.id ? "opacity-40" : ""}`}
            >
              <Panel className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-start gap-2">
                    <span
                      draggable
                      onDragStart={(e) => {
                        e.dataTransfer.effectAllowed = "move";
                        setDraggingId(q.id);
                      }}
                      onDragEnd={() => {
                        setDraggingId(null);
                        setDragOverId(null);
                      }}
                      title="Drag to reorder"
                      aria-label="Drag to reorder"
                      className="mt-0.5 shrink-0 cursor-grab text-dim transition hover:text-paper active:cursor-grabbing"
                    >
                      <svg viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5">
                        <circle cx="9" cy="6" r="1.5" />
                        <circle cx="15" cy="6" r="1.5" />
                        <circle cx="9" cy="12" r="1.5" />
                        <circle cx="15" cy="12" r="1.5" />
                        <circle cx="9" cy="18" r="1.5" />
                        <circle cx="15" cy="18" r="1.5" />
                      </svg>
                    </span>
                    <InlineEditableText
                      value={q.prompt}
                      onSave={(prompt) => renameQuestion(q.id, prompt)}
                      ariaLabel={`Prompt for question ${q.prompt}`}
                      className="min-w-0 flex-1 whitespace-normal break-words font-medium text-paper"
                      inputClassName="min-w-0 flex-1 font-medium"
                    />
                  </div>
                  <button
                    onClick={() => deleteQuestion(q.id)}
                    className="shrink-0 font-mono text-xs text-flare/80 underline decoration-flare/30 underline-offset-4 hover:text-flare"
                  >
                    Delete
                  </button>
                </div>
                {q.type === "free_text" ? (
                  <p className="mt-2 font-mono text-xs uppercase tracking-[0.2em] text-dim">
                    Free text — graded manually
                  </p>
                ) : (
                  <ul className="mt-2 flex flex-col gap-1">
                    {q.options.map((o) => (
                      <li
                        key={o.id}
                        className={`flex items-center gap-2 text-sm ${o.isCorrect ? "text-starlight" : "text-dim"}`}
                      >
                        <span aria-hidden="true">{o.isCorrect ? "★" : "☆"}</span>
                        {o.text}
                      </li>
                    ))}
                  </ul>
                )}

                {q.mediaUrl ? (
                  <div className="mt-3 flex flex-col items-start gap-2">
                    {q.mediaType === "image" ? (
                      <img
                        src={q.mediaUrl}
                        alt=""
                        className="max-h-40 rounded-lg border border-void-3 object-contain"
                      />
                    ) : (
                      <AudioPlayer src={q.mediaUrl} className="w-full" />
                    )}
                    <button
                      type="button"
                      onClick={() => removeMedia(q.id)}
                      className="font-mono text-xs text-flare/80 underline decoration-flare/30 underline-offset-4 hover:text-flare"
                    >
                      Remove media
                    </button>
                  </div>
                ) : (
                  <label className="mt-3 inline-flex w-fit cursor-pointer items-center gap-2 rounded-lg border border-void-3 px-3 py-2 font-mono text-xs text-dim transition hover:border-starlight-dim hover:text-paper">
                    + Add image or audio
                    <input
                      type="file"
                      accept="image/png,image/jpeg,image/webp,image/gif,audio/mpeg,audio/wav,audio/ogg,audio/mp4,audio/webm"
                      className="hidden"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        if (file) uploadMedia(q.id, file);
                        e.target.value = "";
                      }}
                    />
                  </label>
                )}
              </Panel>
            </li>
          ))}
        </ul>
      </section>

      <section className="mt-8">
        <h2 className="mb-3 font-mono text-xs uppercase tracking-[0.2em] text-dim">
          Add a question
        </h2>
        <Panel className="p-5">
          <form onSubmit={addQuestion} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-dim">
                Question type
              </span>
              <div className="flex gap-2">
                {(
                  [
                    { value: "multiple_choice", label: "Multiple choice" },
                    { value: "free_text", label: "Free text" },
                  ] as const
                ).map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setQuestionType(opt.value)}
                    className={`rounded-lg border px-4 py-2 text-sm font-medium transition ${
                      questionType === opt.value
                        ? "border-aurora bg-aurora/10 text-paper"
                        : "border-void-3 bg-void-2/60 text-dim hover:border-starlight-dim"
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>
            <Field
              label="Prompt"
              placeholder="What's the question?"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
            />
            {questionType === "multiple_choice" ? (
              <div className="flex flex-col gap-2">
                <span className="text-xs font-semibold uppercase tracking-[0.2em] text-dim">
                  Options — mark the correct one
                </span>
                {options.map((opt, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <StarToggle checked={opt.isCorrect} onChange={() => setCorrect(i)} label={`Option ${i + 1} is correct`} />
                    <input
                      className="flex-1 rounded-lg border border-void-3 bg-void px-4 py-2.5 text-paper placeholder-dim/60 outline-none transition focus:border-aurora"
                      placeholder={`Option ${i + 1}`}
                      value={opt.text}
                      onChange={(e) => updateOptionText(i, e.target.value)}
                    />
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-dim">
                Players get a text field (up to {MAX_FREE_TEXT_ANSWER_LENGTH} characters) instead of
                options. You'll grade each answer yourself once it's live.
              </p>
            )}
            <div className="flex flex-wrap gap-3">
              {questionType === "multiple_choice" && (
                <Button type="button" variant="ghost" onClick={addOption}>
                  + Add option
                </Button>
              )}
              <Button type="submit">Add question</Button>
            </div>
          </form>
        </Panel>
      </section>
    </main>
  );
}
