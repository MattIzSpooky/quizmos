import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";
import type { Route } from "./+types/admin.quizzes";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";
import { AdminHeader } from "../components/AdminHeader";
import { Button, Field, Panel, Toggle } from "../components/ui";

type Quiz = components["schemas"]["Quiz"];

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Quizzes" }];
}

export default function AdminQuizzes() {
  const ready = useRequireAdmin();
  const navigate = useNavigate();
  const [quizzes, setQuizzes] = useState<Quiz[]>([]);
  const [title, setTitle] = useState("");
  const [timed, setTimed] = useState(true);
  const [loading, setLoading] = useState(true);

  async function load() {
    const { data } = await adminApi.GET("/admin/quizzes");
    if (data) setQuizzes(data);
    setLoading(false);
  }

  useEffect(() => {
    if (ready) load();
  }, [ready]);

  async function createQuiz(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    const { data } = await adminApi.POST("/admin/quizzes", { body: { title: title.trim(), timed } });
    setTitle("");
    if (data) navigate(`/admin/quizzes/${data.id}`);
  }

  async function startGame(quizId: string) {
    const { data } = await adminApi.POST("/admin/games", { body: { quizId } });
    if (data) navigate(`/admin/games/${data.id}`);
  }

  if (!ready) return null;

  return (
    <main className="relative z-0 mx-auto max-w-2xl px-4 py-8 sm:py-12">
      <AdminHeader />
      <h1 className="font-display text-2xl font-semibold text-paper">Your quizzes</h1>

      <Panel className="mt-6 p-5">
        <form onSubmit={createQuiz} className="flex flex-col gap-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <Field
                label="New quiz"
                placeholder="e.g. Friday night trivia"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </div>
            <Button type="submit" className="sm:shrink-0">
              Create
            </Button>
          </div>
          <Toggle
            checked={timed}
            onChange={setTimed}
            label="Timed questions"
            description="Off for a slower, no-pressure pace — no countdown shown to players."
          />
        </form>
      </Panel>

      <div className="mt-8">
        {loading ? (
          <p className="font-mono text-sm text-dim">Charting your quizzes…</p>
        ) : quizzes.length === 0 ? (
          <p className="text-sm text-dim">No quizzes yet — create one above to get started.</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {quizzes.map((q) => (
              <li key={q.id}>
                <Panel className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <Link
                      to={`/admin/quizzes/${q.id}`}
                      className="truncate font-display font-medium text-paper hover:text-starlight"
                    >
                      {q.title}
                    </Link>
                    <p className="font-mono text-xs text-dim">
                      {q.questionCount} question{q.questionCount === 1 ? "" : "s"}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    onClick={() => startGame(q.id)}
                    disabled={q.questionCount === 0}
                    className="shrink-0"
                  >
                    New game
                  </Button>
                </Panel>
              </li>
            ))}
          </ul>
        )}
      </div>
    </main>
  );
}
