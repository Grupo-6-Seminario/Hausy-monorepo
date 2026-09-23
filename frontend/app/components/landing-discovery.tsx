'use client';

import {
  ArrowDownRight,
  ArrowUpRight,
  MessageCircle,
  ScanSearch,
  SlidersHorizontal,
} from 'lucide-react';
import { useEffect, useRef } from 'react';
import './landing-discovery.css';

const inspirations = [
  {
    title: 'Un balcón para bajar un cambio',
    image: 'balcony',
    alt: 'Comedor luminoso abierto a un balcón con plantas',
    detail: 'Aire libre, sin salir de casa.',
    query:
      'Busco un departamento en CABA con balcón y luz natural. Quiero comparar precios, expensas y requisitos de ingreso.',
  },
  {
    title: 'Tu rincón para trabajar',
    image: 'home-office',
    alt: 'Escritorio junto a una ventana con vista a los árboles',
    detail: 'Luz natural y espacio para concentrarte.',
    query:
      'Trabajo desde casa y busco un departamento en CABA con espacio para un escritorio, luz natural y poco ruido.',
  },
  {
    title: 'Espacio para hacerte lugar',
    image: 'living-room',
    alt: 'Living con sillón verde, piso de madera y puertas al balcón',
    detail: 'Un living para compartir todos los días.',
    query:
      'Busco un departamento de dos ambientes en CABA con un living cómodo. Quiero conocer el costo total y las garantías aceptadas.',
  },
];

export function LandingPortrait() {
  return (
    <figure className="landing-portrait">
      <link
        rel="preload"
        as="image"
        href="/images/landing/living-room.webp"
        fetchPriority="high"
      />
      <img
        src="/images/landing/living-room.webp"
        alt="Un living luminoso con balcón y árboles al otro lado"
        width="880"
        height="1100"
        fetchPriority="high"
      />
      <figcaption>
        Imaginá tu próximo lugar. <span>Imagen ilustrativa.</span>
      </figcaption>
    </figure>
  );
}

export function LandingDiscovery({
  onChoose,
}: {
  onChoose: (query: string) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (
      !root.current ||
      !('IntersectionObserver' in window) ||
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    )
      return;
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.setAttribute('data-revealed', 'true');
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12 },
    );
    const sections = root.current.querySelectorAll('[data-landing-reveal]');
    sections.forEach((section) => {
      section.setAttribute('data-revealed', 'false');
      observer.observe(section);
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={root} className="landing-discovery">
      <section
        className="landing-featured"
        aria-labelledby="featured-title"
        data-landing-reveal
      >
        <div className="landing-section-heading">
          <span className="landing-section-icon" aria-hidden="true">
            <ArrowDownRight />
          </span>
          <div>
            <h2 id="featured-title">Avisos destacados</h2>
            <p>
              Ideas para empezar a buscar. Imágenes ilustrativas, no son
              propiedades disponibles.
            </p>
          </div>
        </div>
        <div className="landing-feed">
          {inspirations.map((item) => (
            <article className="landing-home" key={item.image}>
              <div className="landing-home-photo">
                <img
                  src={`/images/landing/${item.image}.webp`}
                  alt={item.alt}
                  width="1000"
                  height="667"
                  loading="lazy"
                />
              </div>
              <div className="landing-home-copy">
                <h3>{item.title}</h3>
                <p>{item.detail}</p>
                <button
                  type="button"
                  data-glow
                  aria-label={`Buscar algo así: ${item.title}`}
                  onClick={() => onChoose(item.query)}
                >
                  Buscar algo así <ArrowUpRight aria-hidden="true" />
                </button>
              </div>
            </article>
          ))}
        </div>
      </section>
      <section
        className="landing-guide"
        aria-labelledby="guide-title"
        data-landing-reveal
      >
        <div className="landing-guide-intro">
          <h2 id="guide-title">Más que una linda foto.</h2>
          <p>Una búsqueda también se trata de lo que necesitás para mudarte.</p>
        </div>
        <dl>
          <div>
            <dt>
              <MessageCircle aria-hidden="true" /> Contanos lo importante
            </dt>
            <dd>
              Zona, presupuesto, garantía y eso a lo que no querés renunciar.
            </dd>
          </div>
          <div>
            <dt>
              <ScanSearch aria-hidden="true" /> Compará con contexto
            </dt>
            <dd>
              Distinguí los datos publicados de lo que todavía falta confirmar.
            </dd>
          </div>
          <div>
            <dt>
              <SlidersHorizontal aria-hidden="true" /> Ajustá a tu ritmo
            </dt>
            <dd>
              Sumá prioridades y seguí la conversación sin empezar de nuevo.
            </dd>
          </div>
        </dl>
      </section>
      <footer className="landing-footer">
        <span>Hausy</span>
        <p>Tu próximo hogar empieza con una buena pregunta.</p>
        <a href="#top">
          Volver al inicio <ArrowUpRight aria-hidden="true" />
        </a>
      </footer>
    </div>
  );
}
