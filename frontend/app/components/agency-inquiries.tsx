import { Inbox, Mail, MessageCircle } from 'lucide-react';

import type { Inquiry } from '@/lib/agency';
import type { EligibilityState } from '@/lib/types';

import { eligibilityMark } from './property-card';

// The searcher's eligibility as the agency reads it; an inquiry is never
// ineligible (CONTEXT.md, Inquiry).
const eligibilityLabel: Record<EligibilityState, string> = {
  eligible: 'Cumple',
  conditionally_eligible: 'Cumple con condiciones',
  unknown: 'Faltan datos',
  ineligible: 'No cumple',
};

// What one requirement gives the searcher, addressed to the agency.
const outcomeLabel: Record<string, string> = {
  met: 'Cumple',
  discretionary: 'A tu criterio',
  near_line: 'Cerca del límite',
  missing: 'Falta el dato',
  unverifiable: 'No verificable',
  unconfirmed: 'A confirmar',
};

const receivedAt = new Intl.DateTimeFormat('es-AR', {
  dateStyle: 'medium',
  timeStyle: 'short',
});

// The agency's inbox: each inquiry exactly as the searcher sent it, newest
// first. Null means the inbox could not be loaded.
export function AgencyInquiries({
  inquiries,
}: {
  inquiries: Inquiry[] | null;
}) {
  return (
    <section
      className="agency-catalog agency-inquiries"
      aria-labelledby="inquiries-title"
    >
      <div className="agency-section-heading">
        <div>
          <h2 id="inquiries-title">Consultas</h2>
          <p>
            Cada consulta trae lo que la persona declaró sobre tus requisitos.
            Hausy no lo verifica.
          </p>
        </div>
      </div>

      {inquiries === null ? (
        <p className="agency-form-error" role="alert">
          No pudimos cargar tus consultas. Recargá la página para intentar de
          nuevo.
        </p>
      ) : inquiries.length === 0 ? (
        <div className="agency-empty">
          <Inbox aria-hidden="true" />
          <h3>Todavía no llegaron consultas</h3>
          <p>Aparecen acá cuando alguien te escribe desde Hausy.</p>
        </div>
      ) : (
        <ul className="agency-inquiry-list">
          {inquiries.map((inquiry, index) => (
            <li key={inquiry.id ?? index} className="agency-inquiry">
              <div>
                <div className="agency-inquiry-heading">
                  <h3>{inquiry.property}</h3>
                  <span
                    className="eligibility-badge"
                    data-state={inquiry.eligibility}
                  >
                    <span className="eligibility-mark" aria-hidden="true">
                      {eligibilityMark[inquiry.eligibility]}
                    </span>
                    {eligibilityLabel[inquiry.eligibility]}
                  </span>
                </div>
                <p className="agency-inquiry-from">
                  <span>{inquiry.name}</span>
                  {inquiry.created_at ? (
                    <time dateTime={inquiry.created_at}>
                      {receivedAt.format(new Date(inquiry.created_at))}
                    </time>
                  ) : null}
                </p>
                {inquiry.requirements.length > 0 ? (
                  <ul
                    className="agency-inquiry-requirements"
                    aria-label="Requisitos de la propiedad"
                  >
                    {inquiry.requirements.map((requirement) => (
                      <li key={requirement.fact}>
                        <span>{requirement.requirement}</span>
                        <span>
                          {requirement.declared.length > 0
                            ? `Declaró: ${requirement.declared.join(', ')}`
                            : 'No lo declaró'}
                        </span>
                        <strong data-outcome={requirement.outcome}>
                          {outcomeLabel[requirement.outcome] ??
                            requirement.outcome}
                        </strong>
                      </li>
                    ))}
                  </ul>
                ) : null}
                {inquiry.searching?.length ? (
                  <p className="agency-inquiry-searching">
                    Busca: {inquiry.searching.join(' · ')}
                  </p>
                ) : null}
                {inquiry.message ? (
                  <blockquote>{inquiry.message}</blockquote>
                ) : null}
              </div>
              <div className="agency-inquiry-contact">
                {inquiry.email ? (
                  <a href={`mailto:${inquiry.email}`} title={inquiry.email}>
                    <Mail aria-hidden="true" />
                    <span>{inquiry.email}</span>
                  </a>
                ) : null}
                {inquiry.phone ? (
                  <a
                    href={`https://wa.me/${inquiry.phone.replace(/\D/g, '')}`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <MessageCircle aria-hidden="true" />
                    WhatsApp {inquiry.phone}
                  </a>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
