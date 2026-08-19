import Link from 'next/link';
import { themeTokens } from '@/components/dashboard-theme';

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";
const MONO = "var(--font-jetbrains-mono), 'JetBrains Mono', monospace";

export default function NotFound() {
  const t = themeTokens(false); // dashboard's own default theme, not persisted across pages either

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
          maxWidth: 380,
        }}
      >
        <div style={{ fontFamily: MONO, fontSize: 13, color: t.text3, marginBottom: 8 }}>404</div>
        <h1 style={{ fontSize: 24, fontWeight: 600, letterSpacing: '-0.01em', margin: 0 }}>Page not found</h1>
        <p style={{ fontSize: 14, color: t.text2, marginTop: 10, marginBottom: 24, lineHeight: 1.5 }}>
          The page you&apos;re looking for doesn&apos;t exist or was moved.
        </p>
        <Link
          href="/"
          style={{
            display: 'inline-block',
            background: t.text,
            color: t.bg,
            borderRadius: 999,
            padding: '10px 22px',
            fontSize: 13.5,
            fontWeight: 600,
            textDecoration: 'none',
          }}
        >
          Back to dashboard
        </Link>
      </div>
    </div>
  );
}
