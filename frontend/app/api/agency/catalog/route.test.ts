import { afterEach, describe, expect, it, vi } from 'vitest';

import { GET, POST } from './route';

const property = {
  id: '14',
  agency: 'Lounge Propiedades',
  source: 'agency',
  contact_count: 7,
  url: 'https://www.zonaprop.com.ar/propiedades/palermo-101.html',
  neighborhood: 'palermo',
  address: 'Humboldt al 1900',
  description: 'Semipiso luminoso al contrafrente.',
  operation: 'alquiler',
  price: { amount: 850, currency: 'USD' },
  expenses: { amount: 120000, currency: 'ARS' },
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('/api/agency/catalog', () => {
  it('forwards the private session as bearer auth when listing the catalog', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(json({ properties: [property] }));
    vi.stubGlobal('fetch', fetchMock);

    const response = await GET(
      new Request('http://localhost/api/agency/catalog', {
        headers: { Cookie: 'hausy_session=secret-token' },
      }),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/agency/catalog',
      expect.objectContaining({
        method: 'GET',
        headers: { Authorization: 'Bearer secret-token' },
      }),
    );
    await expect(response.json()).resolves.toEqual({ properties: [property] });
  });

  it('forwards an added property body without owner or counter fields', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json(property, 201));
    vi.stubGlobal('fetch', fetchMock);
    const input = {
      url: property.url,
      neighborhood: property.neighborhood,
      address: property.address,
      description: property.description,
      operation: property.operation,
      price: property.price,
      expenses: property.expenses,
    };
    const body = JSON.stringify(input);

    const response = await POST(
      new Request('http://localhost/api/agency/catalog', {
        method: 'POST',
        body,
        headers: {
          'Content-Type': 'application/json',
          Cookie: 'hausy_session=secret-token',
        },
      }),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8080/api/agency/catalog',
      expect.objectContaining({ method: 'POST', body }),
    );
    expect(response.status).toBe(201);
  });

  it('answers 401 without a session instead of calling the backend', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await GET(
      new Request('http://localhost/api/agency/catalog'),
    );

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
