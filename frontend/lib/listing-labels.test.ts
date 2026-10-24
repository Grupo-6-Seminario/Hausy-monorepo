import { describe, expect, it } from 'vitest';

import { attributeLabel, requirementLabel } from './listing-labels';

// Every requirement internal/buyer/requirementsFromQuery can produce, read
// the way the searcher would say it.
describe('requirementLabel', () => {
  it.each([
    ['neighborhood', 'palermo', 'Palermo'],
    ['neighborhood', 'villa_crespo', 'Villa Crespo'],
    ['operation', 'alquiler', 'Alquiler'],
    ['operation', 'alquiler_temporal', 'Alquiler temporal'],
    ['operation', 'venta', 'Venta'],
    ['min_price', '500 USD', 'Desde USD 500'],
    ['max_price', '1000 USD', 'Hasta USD 1.000'],
    ['max_price', '850000 ARS', 'Hasta $ 850.000'],
    ['max_expenses', '200000 ARS', 'Expensas hasta $ 200.000'],
    ['min_rooms', '3', 'Desde 3 ambientes'],
    ['min_rooms', '1', 'Desde 1 ambiente'],
    ['max_rooms', '2', 'Hasta 2 ambientes'],
    ['min_bedrooms', '2', 'Desde 2 dormitorios'],
    ['min_bathrooms', '1', 'Desde 1 baño'],
    ['min_parking_spaces', '1', 'Desde 1 cochera'],
    ['max_age_years', '10', 'Hasta 10 años de antigüedad'],
    ['min_total_area_m2', '45', 'Desde 45 m²'],
    ['amenity', 'cochera', 'Con cochera'],
    ['outdoor_space', 'balcon', 'Con balcón'],
    ['natural_light', 'high', 'Luz natural: alta'],
    ['pets_allowed', 'yes', 'Apto mascotas'],
    ['prefiere_amenity', 'pileta', 'Mejor con pileta'],
    ['prefiere_noise_level', 'quiet', 'Mejor: silencioso'],
    ['excluye_exposure', 'contrafrente', 'Sin contrafrente'],
    ['excluye_amenity', 'cochera', 'Sin cochera'],
    // Found in review: slugs carry no accents, and a phrase must fit what it
    // wraps: an adjective is not "sin", a "none" is not "con sin".
    ['neighborhood', 'nunez', 'Núñez'],
    ['neighborhood', 'villa_del_parque', 'Villa del Parque'],
    ['neighborhood', 'velez_sarsfield', 'Vélez Sarsfield'],
    ['excluye_noise_level', 'noisy', 'Que no sea ruidoso'],
    ['excluye_exposure', 'interno', 'Que no sea interno'],
    ['excluye_condition', 'a_refaccionar', 'Que no sea a refaccionar'],
    ['excluye_natural_light', 'low', 'Que no tenga luz natural baja'],
    ['excluye_pets_allowed', 'no', 'Apto mascotas'],
    ['excluye_furnished', 'yes', 'Sin amoblar'],
    ['excluye_outdoor_space', 'none', 'Con espacio exterior'],
    ['prefiere_natural_light', 'high', 'Mejor: luz natural alta'],
    ['prefiere_outdoor_space', 'none', 'Mejor sin espacio exterior'],
    ['outdoor_space', 'none', 'Sin espacio exterior'],
    ['min_total_area_m2', '45.5', 'Desde 45,5 m²'],
  ])('%s %s reads "%s"', (type, value, label) => {
    expect(requirementLabel({ type, value })).toBe(label);
  });

  // A type the frontend does not know yet still reads as a word, never as
  // the backend's field name.
  it('reads an unknown type by its value alone', () => {
    expect(requirementLabel({ type: 'view_type', value: 'al_rio' })).toBe(
      'Al rio',
    );
  });
});

// The listing card and the chips name an attribute the same way.
describe('attributeLabel', () => {
  it.each([
    ['natural_light', 'medium', 'Luz natural: media'],
    ['noise_level', 'quiet', 'Silencioso'],
    ['orientation', 'noreste', 'Orientación noreste'],
    ['exposure', 'frente', 'Al frente'],
    ['outdoor_space', 'balcon_terraza', 'Balcón terraza'],
    ['outdoor_space', 'none', 'Sin espacio exterior'],
    ['condition', 'a_estrenar', 'A estrenar'],
    ['condition', 'muy_bueno', 'Muy buen estado'],
    ['furnished', 'yes', 'Amoblado'],
    ['pets_allowed', 'no', 'No apto mascotas'],
    ['air_conditioning', 'yes', 'Aire acondicionado'],
    ['heating', 'losa_radiante', 'Losa radiante'],
    ['suitable_for', 'profesional', 'Apto profesional'],
    ['amenity', 'sum', 'SUM'],
    ['amenity', 'terraza_comun', 'Terraza común'],
    ['transit_access', 'subte_d', 'Subte D'],
    ['transit_access', 'colectivo', 'Colectivo'],
  ])('%s %s reads "%s"', (type, value, label) => {
    expect(attributeLabel(type, value)).toBe(label);
  });
});
