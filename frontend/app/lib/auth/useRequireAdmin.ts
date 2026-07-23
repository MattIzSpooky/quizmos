import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { getUser, isAdmin } from "./keycloak";

/** Redirects to /admin/login unless the current user holds the quiz-admin role. */
export function useRequireAdmin() {
  const navigate = useNavigate();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    getUser().then((user) => {
      if (!user || user.expired || !isAdmin(user)) {
        navigate("/admin/login", { replace: true });
        return;
      }
      setReady(true);
    });
  }, [navigate]);

  return ready;
}
