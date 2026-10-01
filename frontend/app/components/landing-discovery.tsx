'use client';

import { useEffect, useRef, useState } from 'react';

import type { EligibilityState } from '@/lib/types';

import { eligibilityLabel, eligibilityMark } from './property-card';
import './landing-discovery.css';

type TileKind = 'guarantee' | 'income' | 'confirm' | 'viable' | 'shortlist';

// One fixed scatter of 120 listings, so the picture is the same on every
// visit: three reach the shortlist, the rest drop out step by step.
const tiles: TileKind[] = Array.from({ length: 120 }, (_, index) => {
  if ([17, 58, 93].includes(index)) return 'shortlist';
  const slot = (index * 37 + 11) % 120;
  if (slot < 27) return 'guarantee';
  if (slot < 33) return 'income';
  if (slot < 55) return 'confirm';
  return 'viable';
});

const funnel = [
  { label: '212 avisos en tu zona y presupuesto', count: '212' },
  { label: 'No aceptan tu garantía', count: '−47' },
  { label: 'A confirmar o fuera de tu ingreso', count: '−49' },
  { label: 'Tu terna', count: '3' },
];

/**
 * Walks the funnel one step every 1.9 s while it is on screen. It rests on the
 * last step, which is also what reduced motion and the server render show.
 */
function Funnel() {
  const root = useRef<HTMLDivElement>(null);
  const [step, setStep] = useState(funnel.length - 1);

  useEffect(() => {
    const element = root.current;
    if (
      !element ||
      !('IntersectionObserver' in window) ||
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    )
      return;
    let timer: number | undefined;
    // The funnel itself has no box on wide screens (display: contents), so
    // visibility is measured on the band that holds it.
    const band = element.closest('section') ?? element;
    const observer = new IntersectionObserver(
      ([entry]) => {
        window.clearInterval(timer);
        if (!entry.isIntersecting) return;
        setStep(0);
        timer = window.setInterval(
          () => setStep((current) => (current + 1) % funnel.length),
          1900,
        );
      },
      { threshold: 0.3 },
    );
    observer.observe(band);
    return () => {
      observer.disconnect();
      window.clearInterval(timer);
    };
  }, []);

  return (
    <div ref={root} className="landing-funnel" data-step={step}>
      <ol className="funnel-ledger">
        {funnel.map((row, index) => (
          <li key={row.label} data-reached={index <= step}>
            <span>
              <span className="funnel-swatch" data-row={index} aria-hidden="true" />
              {row.label}
            </span>
            <span className="funnel-count">{row.count}</span>
          </li>
        ))}
      </ol>
      <div className="funnel-tiles" aria-hidden="true">
        {tiles.map((kind, index) => (
          <span key={index} data-kind={kind} />
        ))}
      </div>
    </div>
  );
}

export function LandingDiscovery({ onStart }: { onStart: () => void }) {
  return (
    <div className="landing-discovery">
      <section
        id="como-funciona"
        className="landing-section landing-steps"
        aria-labelledby="steps-title"
      >
        <p className="landing-kicker">Cómo funciona</p>
        <h2 id="steps-title">Tres pasos, en este orden.</h2>
        <ol>
          <li>
            <div className="step-scene step-scene-talk" aria-hidden="true">
              <p className="scene-bubble">2 amb en Palermo, con mucha luz</p>
              <p className="scene-reply">
                Entendí: 2 ambientes en Palermo. Me falta tu presupuesto.
              </p>
            </div>
            <h3>
              <span>01</span> Contás qué buscás
            </h3>
            <p>
              Barrio, ambientes, presupuesto y lo que te importa. Si falta algo
              que cambia el resultado, te lo preguntamos.
            </p>
          </li>
          <li>
            <div className="step-scene step-scene-declare" aria-hidden="true">
              <div className="scene-chips">
                <span data-on="true">✓ Seguro de caución</span>
                <span>+ Garantía propietaria</span>
              </div>
              <div className="scene-bands">
                <span>$1M–2M</span>
                <span data-on="true">$2M–3M</span>
                <span>+$3M</span>
              </div>
            </div>
            <h3>
              <span>02</span> Declarás tu situación
            </h3>
            <p>
              Garantía e ingreso por rangos. Es opcional y lo podés cambiar
              cuando quieras.
            </p>
          </li>
          <li>
            <div className="step-scene step-scene-compare" aria-hidden="true">
              {(
                [
                  ['Gorriti 4800', 'eligible'],
                  ['Guardia Vieja 3900', 'eligible'],
                  ['Palestina 600', 'unknown'],
                ] satisfies [string, EligibilityState][]
              ).map(([address, state]) => (
                <p key={address} className="scene-row">
                  {address}
                  <span className="eligibility-badge" data-state={state}>
                    <span className="eligibility-mark">
                      {eligibilityMark[state]}
                    </span>
                    {eligibilityLabel[state]}
                  </span>
                </p>
              ))}
            </div>
            <h3>
              <span>03</span> Comparás solo lo viable
            </h3>
            <p>
              Te mostramos qué cumple cada opción, de dónde sale cada dato y
              qué conviene confirmar antes de visitar.
            </p>
          </li>
        </ol>
      </section>

      <section className="landing-band" aria-labelledby="funnel-title">
        <div className="landing-section landing-band-inner">
          <div>
            <p className="landing-kicker">De 212 avisos a tu terna</p>
            <h2 id="funnel-title">
              Sacamos lo que no te van a aceptar antes de que lo veas.
            </h2>
          </div>
          <Funnel />
        </div>
      </section>

      <section className="landing-principles" aria-labelledby="principles-title">
        <div id="principios" className="landing-section">
          <p className="landing-kicker">Principios</p>
          <h2 id="principles-title">Lo que no sabemos, te lo decimos.</h2>
          <ul>
            <li>
              <span className="principle-mark" data-shape="circle" aria-hidden="true" />
              <h3>Tus datos se comparten solo si vos querés</h3>
              <p>
                Ninguna inmobiliaria ve tu situación hasta que decidís consultar
                por una propiedad.
              </p>
            </li>
            <li>
              <span className="principle-mark" data-shape="band" aria-hidden="true" />
              <h3>Rangos, no montos</h3>
              <p>
                Para saber si calificás alcanza con una banda de ingreso. No te
                pedimos recibos.
              </p>
            </li>
            <li>
              <span className="principle-mark" data-shape="sources" aria-hidden="true">
                <span />
                <span />
                <span />
              </span>
              <h3>Cada dato dice de dónde sale</h3>
              <p>
                Publicado en el aviso, inferido por nosotros o directamente
                faltante. No completamos lo que no está.
              </p>
            </li>
            <li>
              <span className="principle-mark" data-shape="diamond" aria-hidden="true" />
              <h3>No decidimos por vos</h3>
              <p>Ordenamos y explicamos. La elección es tuya.</p>
            </li>
          </ul>
        </div>
      </section>

      <section className="landing-section landing-cta" aria-labelledby="cta-title">
        <h2 id="cta-title">
          Empezá contando qué buscás. <em>Tarda un minuto.</em>
        </h2>
        <button type="button" onClick={onStart}>
          Empezar búsqueda <span aria-hidden="true">↑</span>
        </button>
      </section>

      <footer className="landing-footer">
        <div className="landing-section">
          <div className="landing-footer-top">
            <p className="landing-wordmark" aria-hidden="true">
              hausy<span />
            </p>
            <p>
              Elegibilidad primero, comparación después. Alquileres en la Ciudad
              de Buenos Aires.
            </p>
          </div>
          <p className="landing-credit">
            Prototipo · Seminario de Integración Profesional, Grupo 2
          </p>
        </div>
      </footer>
    </div>
  );
}
