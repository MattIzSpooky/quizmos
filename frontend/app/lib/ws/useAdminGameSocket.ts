import { useEffect, useRef, useState } from "react";
import { adminApi } from "../api/client";
import { parseServerMessage, type ServerMessage } from "./envelope";
import type { ConnectionStatus } from "./useGameSocket";

const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL ?? "ws://localhost:8080/ws";

/**
 * Connects the admin live-control page to a game's websocket for live
 * updates, replacing what used to be 2s-interval polling. Unlike
 * useGameSocket, connecting here means minting a fresh single-use ticket
 * first (POST /admin/games/{gameId}/ws-ticket) — browsers can't attach the
 * Authorization header to a websocket handshake, so a ticket stands in for
 * the admin's bearer token without ever putting it in the connection URL.
 * A new ticket is minted on every (re)connect attempt, since each one is
 * single-use and expires in seconds.
 */
export function useAdminGameSocket(gameId: string | null, onMessage: (msg: ServerMessage) => void) {
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const socketRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;
  // A ref (not state) so cleanup can suppress the *next* reconnect attempt
  // synchronously, without waiting on a re-render — see useGameSocket.
  const stoppedRef = useRef(false);

  useEffect(() => {
    if (!gameId) return;
    stoppedRef.current = false;
    let retryDelay = 1000;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    async function connect() {
      if (stoppedRef.current || !gameId) return;
      setStatus("connecting");

      const { data } = await adminApi.POST("/admin/games/{gameId}/ws-ticket", {
        params: { path: { gameId } },
      });
      if (stoppedRef.current || !data) return;

      const socket = new WebSocket(`${WS_BASE_URL}/admin/games/${gameId}?ticket=${data.ticket}`);
      socketRef.current = socket;

      socket.onopen = () => {
        retryDelay = 1000;
        setStatus("open");
      };
      socket.onmessage = (event) => {
        onMessageRef.current(parseServerMessage(event.data));
      };
      socket.onclose = () => {
        setStatus("closed");
        if (stoppedRef.current) return;
        retryTimer = setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, 15000);
      };
    }

    connect();
    return () => {
      stoppedRef.current = true;
      clearTimeout(retryTimer);
      socketRef.current?.close();
    };
  }, [gameId]);

  // For callers that know the game has ended (see game.ended in
  // admin.games.$gameId.tsx) — the server closes this connection shortly
  // after anyway, but without this the reconnect-with-backoff logic above
  // would immediately open a new one into a room that's now permanently
  // idle.
  function disconnect() {
    stoppedRef.current = true;
    socketRef.current?.close();
  }

  return { status, disconnect };
}
