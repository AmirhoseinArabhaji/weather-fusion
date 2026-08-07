import type { Metadata } from 'next';
import { Inter, Space_Grotesk } from 'next/font/google';
import './globals.css';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
  display: 'swap',
});

const spaceGrotesk = Space_Grotesk({
  subsets: ['latin'],
  weight: ['500', '600', '700'],
  variable: '--font-space-grotesk',
  display: 'swap',
});

export const metadata: Metadata = {
  title: {
    default: 'Weather Intelligence Platform',
    template: '%s | Weather Intelligence',
  },
  description:
    'Multi-provider weather intelligence platform with AI-powered insights and consensus forecasting.',
  keywords: ['weather', 'forecast', 'AI', 'meteorology', 'analytics'],
  openGraph: {
    type: 'website',
    locale: 'en_US',
    siteName: 'Weather Intelligence Platform',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${spaceGrotesk.variable}`}>
      <body className="bg-gray-950 text-gray-100 antialiased">{children}</body>
    </html>
  );
}
