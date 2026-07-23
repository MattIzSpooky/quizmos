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

  useEffect(() => {
    if (!code) return;
    let closedByClient = false;
    let retryDelay = 1000;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    function connect() {
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
        if (closedByClient) return;
        retryTimer = setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, 15000);
      };
    }

    connect();
    return () => {
      closedByClient = true;
      clearTimeout(retryTimer);
      socketRef.current?.close();
    };
  }, [code]);

  function send(msg: ClientMessage) {
    socketRef.current?.send(encodeClientMessage(msg));
  }

  return { status, send };
}
