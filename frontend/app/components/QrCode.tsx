import { useEffect, useState } from "react";
import QRCode from "qrcode";

/**
 * Renders as plain black-on-white, deliberately breaking from the
 * cosmos palette — a QR code is a scanned instrument, not a themed one,
 * and low-contrast/inverted colors are a real risk to scan reliability
 * across phone cameras.
 */
export function QrCode({ value, size = 152 }: { value: string; size?: number }) {
  const [dataUrl, setDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    QRCode.toDataURL(value, { width: size, margin: 2, color: { dark: "#0a0b14", light: "#ffffff" } })
      .then((url) => {
        if (!cancelled) setDataUrl(url);
      })
      .catch(() => {
        if (!cancelled) setDataUrl(null);
      });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  return (
    <div
      className="flex items-center justify-center rounded-xl bg-white p-2"
      style={{ width: size + 16, height: size + 16 }}
    >
      {dataUrl ? (
        <img src={dataUrl} alt="QR code to join this game" width={size} height={size} />
      ) : (
        <div className="animate-pulse rounded bg-void-3/20" style={{ width: size, height: size }} />
      )}
    </div>
  );
}
