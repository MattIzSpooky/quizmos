import { Link } from "react-router";
import { logout } from "../lib/auth/keycloak";

export function AdminHeader({ back }: { back?: { to: string; label: string } }) {
  return (
    <header className="mb-8 flex items-center justify-between">
      <div>
        <Link to="/admin/quizzes" className="font-display text-lg font-semibold text-paper">
          Quiz<span className="text-starlight">mos</span>{" "}
          <span className="text-sm font-normal text-dim">Admin</span>
        </Link>
        {back && (
          <Link
            to={back.to}
            className="mt-1 block font-mono text-xs text-dim underline decoration-void-3 underline-offset-4 hover:text-aurora"
          >
            ← {back.label}
          </Link>
        )}
      </div>
      <button
        onClick={() => logout()}
        className="font-mono text-xs text-dim underline decoration-void-3 underline-offset-4 hover:text-flare"
      >
        Sign out
      </button>
    </header>
  );
}
