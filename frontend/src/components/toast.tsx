'use client';

import { useEffect, useState } from 'react';

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";

export interface ToastProps {
  message: string;
  dark: boolean;
  durationMs?: number;
  onDone: () => void;
}

// Bottom-centered notice that shrinks its own progress bar down to 0 and
// then dismisses itself — the bar tracks this toast's own display timer, not
// any server-side retry window (that can be an hour, useless to visualize).
export function Toast({ message, dark, durationMs = 6000, onDone }: ToastProps) {
  const [remaining, setRemaining] = useState(1);

  useEffect(() => {
    const start = Date.now();
    const id = setInterval(() => {
      const left = Math.max(0, 1 - (Date.now() - start) / durationMs);
      setRemaining(left);
      if (left <= 0) {
        clearInterval(id);
        onDone();
      }
    }, 50);
    return () => clearInterval(id);
  }, [durationMs, onDone]);

  return (
    <div
      style={{
        position: 'fixed',
        left: '50%',
        bottom: 24,
        transform: 'translateX(-50%)',
        zIndex: 50,
        minWidth: 280,
        maxWidth: 420,
        background: dark ? 'oklch(0.24 0.03 265)' : 'oklch(1 0 0)',
        border: `1px solid ${dark ? 'oklch(0.4 0.03 265)' : 'oklch(0.89 0.014 260)'}`,
        borderRadius: 14,
        boxShadow: '0 12px 32px oklch(0 0 0 / 0.35)',
        overflow: 'hidden',
        fontFamily: SPACE_GROTESK,
      }}
    >
      <div style={{ padding: '12px 16px', fontSize: 13, lineHeight: 1.4, color: dark ? 'oklch(0.94 0.01 265)' : 'oklch(0.24 0.02 265)' }}>
        {message}
      </div>
      <div style={{ height: 3, background: dark ? 'oklch(0.35 0.03 265)' : 'oklch(0.92 0.014 260)' }}>
        <div style={{ height: '100%', width: `${remaining * 100}%`, background: 'oklch(0.62 0.19 25)' }} />
      </div>
    </div>
  );
}
