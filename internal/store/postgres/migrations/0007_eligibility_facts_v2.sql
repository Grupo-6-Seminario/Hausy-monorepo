-- The qualification form reads its questions from this table (specs/004,
-- phase 1), so each fact carries its label, priority, cardinality and order,
-- and each choice its label.
ALTER TABLE eligibility_facts
    ADD COLUMN label    text    NOT NULL DEFAULT '',
    ADD COLUMN priority text    NOT NULL DEFAULT 'low' CHECK (priority IN ('high', 'medium', 'low')),
    ADD COLUMN multiple boolean NOT NULL DEFAULT false,
    ADD COLUMN position integer NOT NULL DEFAULT 0;

UPDATE eligibility_facts SET label = 'Garantía', priority = 'high', multiple = true, position = 1,
    choices = '[{"value": "propietaria", "label": "Garantía propietaria"},
                {"value": "caucion", "label": "Seguro de caución"},
                {"value": "recibos_garante", "label": "Recibos de sueldo de un garante"}]'
    WHERE name = 'guarantee';

UPDATE eligibility_facts SET label = 'Ingresos mensuales', priority = 'medium', position = 4,
    choices = '[{"value": "0-1000000", "label": "Hasta $1.000.000"},
                {"value": "1000000-2000000", "label": "$1.000.000 a $2.000.000"},
                {"value": "2000000-3000000", "label": "$2.000.000 a $3.000.000"},
                {"value": "3000000-", "label": "Más de $3.000.000"}]'
    WHERE name = 'income_band';

UPDATE eligibility_facts SET label = '¿Ya cotizaste un seguro de caución?', priority = 'medium', position = 5,
    choices = '[{"value": "yes", "label": "Sí"}, {"value": "no", "label": "No"}]'
    WHERE name = 'caucion_quoted';

INSERT INTO eligibility_facts (name, admissible, label, priority, multiple, position, choices) VALUES
    ('income_documented', true, '¿Podés comprobar tus ingresos?', 'high', false, 2,
     '[{"value": "yes", "label": "Sí"}, {"value": "no", "label": "No"}]'),
    ('pets', true, 'Mascotas', 'high', true, 3,
     '[{"value": "none", "label": "No tengo"}, {"value": "dog", "label": "Perro"},
       {"value": "cat", "label": "Gato"}, {"value": "other", "label": "Otra"}]');
