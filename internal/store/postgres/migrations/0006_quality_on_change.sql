-- Keep the publication gate at the shared persistence seam: both scraper
-- upserts and agency edits must return changed facts to pending review.
ALTER TABLE listings ALTER COLUMN quality_status SET DEFAULT 'pending';

CREATE FUNCTION reset_listing_quality_on_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.source, NEW.url, NEW.neighborhood, NEW.agency_id, NEW.address,
        NEW.description, NEW.operation, NEW.price_amount, NEW.price_currency,
        NEW.expenses_amount, NEW.expenses_currency, NEW.total_area_m2,
        NEW.covered_area_m2, NEW.rooms, NEW.bedrooms, NEW.bathrooms,
        NEW.parking_spaces, NEW.age_years, NEW.floor, NEW.parsed_at,
        NEW.parser_model)
       IS DISTINCT FROM
       (OLD.source, OLD.url, OLD.neighborhood, OLD.agency_id, OLD.address,
        OLD.description, OLD.operation, OLD.price_amount, OLD.price_currency,
        OLD.expenses_amount, OLD.expenses_currency, OLD.total_area_m2,
        OLD.covered_area_m2, OLD.rooms, OLD.bedrooms, OLD.bathrooms,
        OLD.parking_spaces, OLD.age_years, OLD.floor, OLD.parsed_at,
        OLD.parser_model) THEN
        NEW.quality_status := 'pending';
        NEW.quality_evidence := NULL;
        NEW.quality_checked_at := NULL;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER listings_reset_quality_on_change
BEFORE UPDATE ON listings FOR EACH ROW
EXECUTE FUNCTION reset_listing_quality_on_change();
