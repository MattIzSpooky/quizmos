import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";
import type { Route } from "./+types/admin.quizzes";
import { adminApi } from "../lib/api/client";
import { useRequireAdmin } from "../lib/auth/useRequireAdmin";
import type { components } from "../lib/api/schema.gen";

type Quiz = components["schemas"]["Quiz"];

export function meta({}: Route.MetaArgs) {
  return [{ title: "Quizmos Admin — Quizzes" }];
}

export default function AdminQuizzes() {
  const ready = useRequireAdmin();
  const navigate = useNavigate();
  const [quizzes, setQuizzes] = useState<Quiz[]>([]);
  const [title, setTitle] = useState("");
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
    const { data } = await adminApi.POST("/admin/quizzes", { body: { title: title.trim() } });
    setTitle("");
    if (data) navigate(`/admin/quizzes/${data.id}`);
  }

  async function startGame(quizId: string) {
    const { data } = await adminApi.POST("/admin/games", { body: { quizId } });
    if (data) navigate(`/admin/games/${data.id}`);
  }

  if (!ready) return null;

  return (
    <main className="max-w-2xl mx-auto p-6 flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Your quizzes</h1>

      <form onSubmit={createQuiz} className="flex gap-2">
        <input
          className="border rounded px-3 py-2 flex-1"
          placeholder="New quiz title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <button className="bg-black text-white rounded px-4 py-2">Create</button>
      </form>

      {loading ? (
        <p>Loading…</p>
      ) : quizzes.length === 0 ? (
        <p className="text-gray-500">No quizzes yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {quizzes.map((q) => (
            <li key={q.id} className="border rounded px-4 py-3 flex items-center justify-between">
              <div>
                <Link to={`/admin/quizzes/${q.id}`} className="font-medium underline">
                  {q.title}
                </Link>
                <p className="text-sm text-gray-500">{q.questionCount} question(s)</p>
              </div>
              <button
                onClick={() => startGame(q.id)}
                disabled={q.questionCount === 0}
                className="border rounded px-3 py-1 disabled:opacity-50"
              >
                New game
              </button>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
