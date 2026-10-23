-- No rule reads a caución quote, so the form stops asking it
-- (specs/007-income-proof-rule). Saved answers go first: they reference the
-- fact row. No listing rule names the fact.
DELETE FROM user_qualifications WHERE fact = 'caucion_quoted';
DELETE FROM eligibility_facts WHERE name = 'caucion_quoted';
