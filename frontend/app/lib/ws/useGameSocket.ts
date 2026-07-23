import { useEffect, useRef, useState } from "react";
import { getClientId } from "../client-id";
import { parseServerMessage, encodeClientMessage, type ServerMessage, type ClientMessage } from "./envelope";

const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL ?? "ws://localhost:8080/ws";

export type ConnectionStatus = "connecting" | "open" | "closed";

/**
 * Connects to a game's live-play channel and calls onMessage for every
 * event. Reconnects with backoff on unexpected close; a normal unmount
 * closes the socket for good.
 */
export function useGameSocket(code: string | null, onMessage: (msg: ServerMessage) => void) {
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const socketRef = useRef<WebSocket | null>(null);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;
  // A ref (not state) so `disconnect` can suppress the *next* reconnect
  // attempt synchronously, without waiting on a re-render.
  const stoppedRef = useRef(false);

  useEffect(() => {
    if (!code) return;
    stoppedRef.current = false;
    let retryDelay = 1000;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    function connect() {
      if (stoppedRef.current) return;
      const clientId = getClientId();
      const socket = new WebSocket(`${WS_BASE_URL}/games/${code}?client_id=${clientId}`);
      socketRef.current = socket;
      setStatus("connecting");

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
  }, [code]);

  function send(msg: ClientMessage) {
    socketRef.current?.send(encodeClientMessage(msg));
  }

  // For cases where the server closing the connection means "stay
  // closed" (e.g. the player was kicked) rather than "reconnect me".
  function disconnect() {
    stoppedRef.current = true;
    socketRef.current?.close();
  }

  return { status, send, disconnect };
}
