-- Pets rules now name the animals a listing refuses ("gatos no" admits dogs),
-- so the form asks for exactly those animals: dog, cat or none.
UPDATE eligibility_facts SET choices =
    '[{"value": "none", "label": "Ninguna"}, {"value": "dog", "label": "Perro"},
      {"value": "cat", "label": "Gato"}]'
    WHERE name = 'pets';

DELETE FROM user_qualifications WHERE fact = 'pets' AND value NOT IN ('none', 'dog', 'cat');
