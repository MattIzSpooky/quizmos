import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("play/:code", "routes/play.$code.tsx"),
  route("admin/login", "routes/admin.login.tsx"),
  route("admin/callback", "routes/admin.callback.tsx"),
  route("admin/quizzes", "routes/admin.quizzes.tsx"),
  route("admin/quizzes/:quizId", "routes/admin.quizzes.$quizId.tsx"),
  route("admin/games/:gameId", "routes/admin.games.$gameId.tsx"),
] satisfies RouteConfig;
