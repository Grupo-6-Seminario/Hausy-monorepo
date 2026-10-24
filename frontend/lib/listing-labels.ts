// Spanish names for what a listing has and what a search asks for. The
// result card and the "Tu búsqueda" chips share them, so a quality reads the
// same in both places.

export function formatMoney(
  amount?: number | null,
  currency?: string | null,
): string {
  if (amount == null) return 'A consultar';
  const formattedNumber = new Intl.NumberFormat('es-AR', {
    maximumFractionDigits: 0,
  }).format(amount);
  return currency === 'USD' ? `USD ${formattedNumber}` : `$ ${formattedNumber}`;
}

// humanize reads a slug as words: "villa_crespo" -> "Villa crespo".
function humanize(slug: string): string {
  const words = slug.replaceAll('_', ' ');
  return words.charAt(0).toUpperCase() + words.slice(1);
}

// lowerFirst puts a label inside a phrase ("Con cochera") but keeps an
// acronym as written ("Con SUM").
function lowerFirst(label: string): string {
  return label.length > 1 && label[1] === label[1].toLowerCase()
    ? label.charAt(0).toLowerCase() + label.slice(1)
    : label;
}

// The listing vocabulary (internal/listing.Vocabulary) in Spanish.
const attributeValues: Record<string, Record<string, string>> = {
  natural_light: {
    high: 'Luz natural: alta',
    medium: 'Luz natural: media',
    low: 'Luz natural: baja',
  },
  noise_level: {
    quiet: 'Silencioso',
    moderate: 'Ruido moderado',
    noisy: 'Ruidoso',
  },
  exposure: {
    frente: 'Al frente',
    contrafrente: 'Contrafrente',
    interno: 'Interno',
    lateral: 'Lateral',
  },
  outdoor_space: {
    balcon: 'Balcón',
    balcon_terraza: 'Balcón terraza',
    terraza: 'Terraza',
    patio: 'Patio',
    jardin: 'Jardín',
    none: 'Sin espacio exterior',
  },
  condition: {
    a_estrenar: 'A estrenar',
    excelente: 'Excelente estado',
    muy_bueno: 'Muy buen estado',
    bueno: 'Buen estado',
    a_refaccionar: 'A refaccionar',
  },
  furnished: { yes: 'Amoblado', no: 'Sin amoblar' },
  pets_allowed: { yes: 'Apto mascotas', no: 'No apto mascotas' },
  air_conditioning: { yes: 'Aire acondicionado', no: 'Sin aire acondicionado' },
  heating: {
    central: 'Calefacción central',
    individual: 'Calefacción individual',
    losa_radiante: 'Losa radiante',
    none: 'Sin calefacción',
  },
  amenity: {
    sum: 'SUM',
    solarium: 'Solárium',
    terraza_comun: 'Terraza común',
  },
};

const attributeTypes = new Set([
  ...Object.keys(attributeValues),
  'orientation',
  'suitable_for',
  'transit_access',
]);

// attributeLabel names one listing attribute. A value the vocabulary does not
// list still reads as words, never as the field name.
export function attributeLabel(type: string, value: string): string {
  const known = attributeValues[type]?.[value];
  if (known) return known;
  switch (type) {
    case 'orientation':
      return `Orientación ${value}`;
    case 'suitable_for':
      return `Apto ${value}`;
    case 'transit_access':
      return value.startsWith('subte_')
        ? `Subte ${value.slice('subte_'.length).toUpperCase()}`
        : humanize(value);
  }
  return humanize(value);
}

// Things that read "Con …" when asked for: a building amenity, a balcony.
const withPreposition = new Set(['amenity', 'outdoor_space']);

const counts: Record<string, [string, string, string]> = {
  min_rooms: ['Desde', 'ambiente', 'ambientes'],
  max_rooms: ['Hasta', 'ambiente', 'ambientes'],
  min_bedrooms: ['Desde', 'dormitorio', 'dormitorios'],
  min_bathrooms: ['Desde', 'baño', 'baños'],
  min_parking_spaces: ['Desde', 'cochera', 'cocheras'],
  max_age_years: ['Hasta', 'año de antigüedad', 'años de antigüedad'],
};

// The barrios whose name is more than their slug with capitals: an accent,
// or a lowercase "del". Every other slug reads by capitalizing its words.
const barrioNames: Record<string, string> = {
  agronomia: 'Agronomía',
  constitucion: 'Constitución',
  nunez: 'Núñez',
  san_cristobal: 'San Cristóbal',
  san_nicolas: 'San Nicolás',
  velez_sarsfield: 'Vélez Sarsfield',
  villa_del_parque: 'Villa del Parque',
  villa_ortuzar: 'Villa Ortúzar',
  villa_pueyrredon: 'Villa Pueyrredón',
};

// money reads the backend's "<amount> <currency>" value.
function money(value: string): string {
  const [amount, currency] = value.split(' ');
  return formatMoney(Number(amount), currency);
}

// inline puts an attribute's label inside a phrase: "luz natural alta".
function inline(label: string): string {
  return lowerFirst(label).replace(': ', ' ');
}

// A label that already denies ("Sin calefacción", "No apto mascotas") takes
// no "Con" in front.
function denies(label: string): boolean {
  return /^(Sin|No) /.test(label);
}

// What reads "Sin …" when excluded; adjectives read "Que no sea …".
const excludedAsNoun = new Set([
  'amenity',
  'outdoor_space',
  'transit_access',
  'heating',
]);
const excludedAsHaving = new Set(['natural_light', 'orientation']);
const yesNo = new Set(['furnished', 'pets_allowed', 'air_conditioning']);
const excludedNone: Record<string, string> = {
  outdoor_space: 'Con espacio exterior',
  heating: 'Con calefacción',
};

// excluded names what a search rules out: "Sin contrafrente", "Que no sea
// ruidoso"; ruling out "none" or "no" asks for the opposite.
function excluded(attribute: string, value: string): string {
  if (yesNo.has(attribute)) {
    return attributeLabel(attribute, value === 'yes' ? 'no' : 'yes');
  }
  if (value === 'none' && excludedNone[attribute]) {
    return excludedNone[attribute];
  }
  const label = inline(attributeLabel(attribute, value));
  if (excludedAsNoun.has(attribute) || value === 'contrafrente') {
    return `Sin ${label}`;
  }
  return `Que no ${excludedAsHaving.has(attribute) ? 'tenga' : 'sea'} ${label}`;
}

// preferred names what a search would rather have: "Mejor con pileta",
// "Mejor: silencioso".
function preferred(attribute: string, value: string): string {
  const label = attributeLabel(attribute, value);
  if (denies(label)) return `Mejor ${lowerFirst(label)}`;
  return withPreposition.has(attribute)
    ? `Mejor con ${inline(label)}`
    : `Mejor: ${inline(label)}`;
}

// requirementLabel names one condition a search is using, from the
// requirements the backend reports (internal/buyer requirementsFromQuery):
// "Hasta USD 1.000", "Desde 3 ambientes", "Villa Crespo", "Sin contrafrente".
export function requirementLabel({
  type,
  value,
}: {
  type: string;
  value: string;
}): string {
  if (counts[type]) {
    const [bound, one, many] = counts[type];
    return `${bound} ${value} ${value === '1' ? one : many}`;
  }
  switch (type) {
    case 'neighborhood':
      return (
        barrioNames[value] ??
        value
          .split('_')
          .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
          .join(' ')
      );
    case 'operation':
      return humanize(value);
    case 'min_price':
      return `Desde ${money(value)}`;
    case 'max_price':
      return `Hasta ${money(value)}`;
    case 'max_expenses':
      return `Expensas hasta ${money(value)}`;
    case 'min_total_area_m2':
      return `Desde ${Number(value).toLocaleString('es-AR')} m²`;
  }
  if (type.startsWith('excluye_')) {
    return excluded(type.slice('excluye_'.length), value);
  }
  if (type.startsWith('prefiere_')) {
    return preferred(type.slice('prefiere_'.length), value);
  }
  if (attributeTypes.has(type)) {
    const label = attributeLabel(type, value);
    return withPreposition.has(type) && !denies(label)
      ? `Con ${inline(label)}`
      : label;
  }
  return humanize(value);
}
