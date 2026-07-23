import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/admin.quizzes.$quizId";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";

type QuizDetail = components["schemas"]["QuizDetail"];

const EMPTY_OPTIONS = [
  { text: "", isCorrect: true },
  { text: "", isCorrect: false },
];

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Edit quiz" }];
}

export default function AdminQuizDetail({ params }: Route.ComponentProps) {
  const ready = useRequireAdmin();
  const navigate = useNavigate();
  const quizId = params.quizId;
  const [quiz, setQuiz] = useState<QuizDetail | null>(null);
  const [prompt, setPrompt] = useState("");
  const [options, setOptions] = useState(EMPTY_OPTIONS);

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
    if (!prompt.trim() || options.some((o) => !o.text.trim())) return;
    await adminApi.POST("/admin/quizzes/{quizId}/questions", {
      params: { path: { quizId } },
      body: { type: "multiple_choice", prompt: prompt.trim(), timeLimitSeconds: 30, points: 1000, options },
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

  async function startGame() {
    const { data } = await adminApi.POST("/admin/games", { body: { quizId } });
    if (data) navigate(`/admin/games/${data.id}`);
  }

  if (!ready || !quiz) return null;

  return (
    <main className="max-w-2xl mx-auto p-6 flex flex-col gap-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{quiz.title}</h1>
        <button
          onClick={startGame}
          disabled={quiz.questions.length === 0}
          className="bg-black text-white rounded px-4 py-2 disabled:opacity-50"
        >
          Start a new game
        </button>
      </div>

      <section className="flex flex-col gap-3">
        <h2 className="font-semibold">Questions</h2>
        {quiz.questions.length === 0 && <p className="text-gray-500">No questions yet.</p>}
        <ul className="flex flex-col gap-2">
          {quiz.questions.map((q) => (
            <li key={q.id} className="border rounded px-4 py-3">
              <div className="flex items-center justify-between">
                <p className="font-medium">{q.prompt}</p>
                <button onClick={() => deleteQuestion(q.id)} className="text-sm text-red-600">
                  Delete
                </button>
              </div>
              <ul className="text-sm text-gray-600 mt-1">
                {q.options.map((o) => (
                  <li key={o.id}>
                    {o.isCorrect ? "✓" : "·"} {o.text}
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      </section>

      <section className="flex flex-col gap-3 border-t pt-6">
        <h2 className="font-semibold">Add a question</h2>
        <form onSubmit={addQuestion} className="flex flex-col gap-3">
          <input
            className="border rounded px-3 py-2"
            placeholder="Question prompt"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
          />
          {options.map((opt, i) => (
            <div key={i} className="flex items-center gap-2">
              <input
                type="radio"
                name="correct"
                checked={opt.isCorrect}
                onChange={() => setCorrect(i)}
                title="Correct answer"
              />
              <input
                className="border rounded px-3 py-2 flex-1"
                placeholder={`Option ${i + 1}`}
                value={opt.text}
                onChange={(e) => updateOptionText(i, e.target.value)}
              />
            </div>
          ))}
          <div className="flex gap-2">
            <button type="button" onClick={addOption} className="border rounded px-3 py-1 text-sm">
              + Add option
            </button>
            <button className="bg-black text-white rounded px-4 py-2">Add question</button>
          </div>
        </form>
      </section>
    </main>
  );
}
