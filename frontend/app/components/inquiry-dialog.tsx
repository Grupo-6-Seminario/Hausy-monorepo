'use client';

import { type SyntheticEvent, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type { Inquiry } from '@/lib/agency';
import {
  type InquiryContact,
  InquiryRequestError,
  previewInquiry,
  sendInquiry,
} from '@/lib/inquiry';
import type { Qualification } from '@/lib/types';

// What the searcher reads beside each requirement the agency will see.
const searcherOutcome: Record<string, string> = {
  met: 'Cumplís',
  discretionary: 'Decide la inmobiliaria',
  near_line: 'Depende de tu monto exacto',
  missing: 'Falta este dato',
  unverifiable: 'No se puede verificar',
  unconfirmed: 'A confirmar',
};

interface InquiryDialogProps {
  listingId: string;
  title: string;
  agency?: string;
  declared: Qualification;
  searching: string[];
  onClose: () => void;
}

// InquiryDialog shows the payload the server would deliver before anything
// leaves (CONTEXT.md, Inquiry): the searcher sends exactly what they read.
export function InquiryDialog({
  listingId,
  title,
  agency,
  declared,
  searching,
  onClose,
}: InquiryDialogProps) {
  const recipient = agency || 'la inmobiliaria';
  const [preview, setPreview] = useState<Inquiry | null>(null);
  const [contact, setContact] = useState<InquiryContact>({
    name: '',
    email: '',
    phone: '',
    message: '',
  });
  const [error, setError] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [sent, setSent] = useState<Inquiry | null>(null);
  // One id per dialog, so retrying a failed send is the same inquiry.
  const [inquiryId] = useState(() => crypto.randomUUID());
  const firstField = useRef<HTMLInputElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  // The parent rebuilds these on every render; their content is what matters.
  const situation = JSON.stringify([declared, searching]);

  useEffect(() => {
    const modal = dialog.current;
    modal?.showModal();
    firstField.current?.focus();
    return () => modal?.close();
  }, []);

  useEffect(() => {
    let active = true;
    const [qualification, phrases] = JSON.parse(situation) as [
      Qualification,
      string[],
    ];
    previewInquiry(listingId, qualification, phrases)
      .then((inquiry) => {
        if (active) setPreview(inquiry);
      })
      .catch((cause) => {
        if (active) {
          setError(
            cause instanceof Error
              ? cause.message
              : 'No pudimos preparar la consulta.',
          );
        }
      });
    return () => {
      active = false;
    };
  }, [listingId, situation]);

  async function send(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isSending || !preview) return;
    setIsSending(true);
    setError('');
    try {
      const receipt = await sendInquiry(
        listingId,
        inquiryId,
        contact,
        declared,
        searching,
        preview.preview_revision ?? '',
      );
      setSent(receipt);
    } catch (cause) {
      if (
        cause instanceof InquiryRequestError &&
        cause.code === 'preview_changed'
      ) {
        setPreview(null);
        try {
          setPreview(await previewInquiry(listingId, declared, searching));
        } catch (refreshError) {
          setError(
            refreshError instanceof Error
              ? refreshError.message
              : 'No pudimos actualizar la consulta.',
          );
          return;
        }
      }
      setError(
        cause instanceof Error
          ? cause.message
          : 'No pudimos enviar la consulta. Probá de nuevo.',
      );
    } finally {
      setIsSending(false);
    }
  }

  function field(name: keyof InquiryContact, value: string) {
    setContact((current) => ({ ...current, [name]: value }));
  }

  // The native modal keeps focus inside and makes the page behind it inert.
  return createPortal(
    <>
      <dialog
        ref={dialog}
        className="agency-editor inquiry-dialog"
        aria-modal="true"
        aria-labelledby="inquiry-title"
        onCancel={(event) => {
          event.preventDefault();
          onClose();
        }}
      >
        <div className="agency-editor-heading">
          <div>
            <p className="eyebrow">Consulta a {recipient}</p>
            <h2 id="inquiry-title">{title}</h2>
          </div>
          <Button type="button" variant="ghost" onClick={onClose}>
            {sent ? 'Cerrar' : 'Cancelar'}
          </Button>
        </div>

        {sent ? (
          <output className="inquiry-sent">
            Listo, tu consulta le llegó a {sent.agency || recipient}. Te va a
            responder{' '}
            {sent.email ? `a ${sent.email}` : `por WhatsApp al ${sent.phone}`}.
          </output>
        ) : (
          <form className="agency-property-form" onSubmit={send}>
            <div className="agency-field agency-field-wide">
              <label htmlFor="inquiry-name">Tu nombre</label>
              <Input
                ref={firstField}
                id="inquiry-name"
                required
                maxLength={80}
                autoComplete="given-name"
                value={contact.name}
                onChange={(event) => field('name', event.target.value)}
              />
            </div>
            <div className="agency-field">
              <label htmlFor="inquiry-email">Email</label>
              <Input
                id="inquiry-email"
                type="email"
                autoComplete="email"
                value={contact.email}
                onChange={(event) => field('email', event.target.value)}
              />
            </div>
            <div className="agency-field">
              <label htmlFor="inquiry-phone">WhatsApp</label>
              <Input
                id="inquiry-phone"
                type="tel"
                autoComplete="tel"
                placeholder="+54 9 11 …"
                aria-describedby="inquiry-phone-help"
                value={contact.phone}
                onChange={(event) => field('phone', event.target.value)}
              />
              <p id="inquiry-phone-help">Incluí + y el código de país.</p>
            </div>
            <div className="agency-field agency-field-wide">
              <label htmlFor="inquiry-message">Mensaje (opcional)</label>
              <Textarea
                id="inquiry-message"
                maxLength={1000}
                rows={3}
                value={contact.message}
                onChange={(event) => field('message', event.target.value)}
              />
            </div>

            <section
              className="agency-field-wide inquiry-preview"
              aria-labelledby="inquiry-preview-title"
            >
              <h3 id="inquiry-preview-title">Lo que recibe {recipient}</h3>
              {preview ? (
                <>
                  {preview.requirements.length > 0 ? (
                    <ul>
                      {preview.requirements.map((requirement) => (
                        <li key={requirement.fact}>
                          <span>{requirement.requirement}</span>
                          <span>
                            {requirement.declared.length > 0
                              ? `Declaraste: ${requirement.declared.join(', ')}`
                              : 'No lo declaraste'}
                          </span>
                          <strong data-outcome={requirement.outcome}>
                            {searcherOutcome[requirement.outcome] ??
                              requirement.outcome}
                          </strong>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p>Esta propiedad no declaró requisitos para alquilar.</p>
                  )}
                  {preview.searching?.length ? (
                    <p>Lo que buscás: {preview.searching.join(' · ')}</p>
                  ) : null}
                </>
              ) : error ? null : (
                <p aria-live="polite">Preparando lo que se envía…</p>
              )}
              <p className="inquiry-privacy">
                Además van tu nombre, tu contacto y tu mensaje. Nada más: ni tu
                presupuesto ni las otras propiedades que viste.
              </p>
            </section>

            {error ? (
              <p className="agency-form-error" role="alert">
                {error}
              </p>
            ) : null}
            <div className="agency-form-actions">
              <Button type="submit" disabled={isSending || !preview}>
                {isSending ? 'Enviando…' : 'Enviar consulta'}
              </Button>
            </div>
          </form>
        )}
      </dialog>
    </>,
    document.body,
  );
}
