import type { Metadata } from 'next';
import { DM_Sans, Geist_Mono } from 'next/font/google';

import { themeBootScript } from '@/lib/theme';

import './globals.css';

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL ?? 'http://localhost:3000';

const hausySans = DM_Sans({
  variable: '--font-hausy-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: 'Hausy | Encontrá el lugar que encaja con tu vida',
  description:
    'Búsqueda inmobiliaria que entiende requisitos, preferencias y concesiones antes de comparar opciones.',
  openGraph: {
    title: 'Hausy | Encontrá el lugar que encaja con tu vida',
    description:
      'Búsqueda inmobiliaria que entiende requisitos, preferencias y concesiones antes de comparar opciones.',
    locale: 'es_AR',
    type: 'website',
    images: [
      {
        url: '/og.png',
        width: 1200,
        height: 630,
        alt: 'Hausy, búsqueda inmobiliaria personal',
      },
    ],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Hausy | Encontrá el lugar que encaja con tu vida',
    description:
      'Búsqueda inmobiliaria que entiende requisitos, preferencias y concesiones antes de comparar opciones.',
    images: ['/og.png'],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    // The boot script writes `data-theme` on this element before first paint,
    // so a remembered dark page never flashes light. React sees the attribute
    // it did not render; that difference is the point, not a mismatch.
    <html lang="es" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeBootScript }} />
      </head>
      <body className={`${hausySans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
