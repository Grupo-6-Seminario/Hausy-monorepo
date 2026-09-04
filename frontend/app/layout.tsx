import type { Metadata } from 'next';
import { DM_Sans, Geist_Mono } from 'next/font/google';

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
    <html lang="es">
      <body className={`${hausySans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
