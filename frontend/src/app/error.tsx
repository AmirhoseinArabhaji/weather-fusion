'use client';

import { themeTokens } from '@/components/dashboard-theme';

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";
const MONO = "var(--font-jetbrains-mono), 'JetBrains Mono', monospace";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const t = themeTokens(false);

  return (
    <div
      style={{
        minHeight: '100vh',
        background: t.bg,
        backgroundImage: t.ambient,
        color: t.text,
        fontFamily: SPACE_GROTESK,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 16,
        padding: 24,
        textAlign: 'center',
      }}
    >
      <div
        style={{
          background: t.glass,
          border: `1px solid ${t.border}`,
          borderRadius: 24,
          padding: '40px 48px',
          maxWidth: 420,
        }}
      >
        <div style={{ fontFamily: MONO, fontSize: 13, color: t.text3, marginBottom: 8 }}>error</div>
        <h1 style={{ fontSize: 24, fontWeight: 600, letterSpacing: '-0.01em', margin: 0 }}>Something went wrong</h1>
        <p style={{ fontSize: 14, color: t.text2, marginTop: 10, marginBottom: 24, lineHeight: 1.5 }}>
          {error.message || 'Unexpected error while rendering the page.'}
        </p>
        <button
          onClick={reset}
          style={{
            background: t.text,
            color: t.bg,
            border: 'none',
            borderRadius: 999,
            padding: '10px 22px',
            fontSize: 13.5,
            fontWeight: 600,
            cursor: 'pointer',
            fontFamily: SPACE_GROTESK,
          }}
        >
          Try again
        </button>
      </div>
    </div>
  );
}
