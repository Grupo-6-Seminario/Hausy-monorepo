package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// probeScript reports what a listing page actually exposes, so the extraction
// selectors are written against the real document instead of a guess. ZonaProp
// ships obfuscated class names that rotate, so anything structured the page
// already carries (JSON-LD, an embedded state blob, stable data attributes) is
// a far better anchor than a CSS class.
const probeScript = `() => {
	const dump = (root, depth) => {
		if (!root || depth > 3) return null;
		return {
			tag: root.tagName, id: root.id || '', qa: root.getAttribute('data-qa') || '',
			cls: (root.className && root.className.toString ? root.className.toString() : '').slice(0, 60),
			text: (root.innerText || '').replace(/\s+/g, ' ').trim().slice(0, 120),
			children: Array.from(root.children).map(c => dump(c, depth + 1)).filter(Boolean),
		};
	};

	const pick = sel => {
		const el = document.querySelector(sel);
		return el ? dump(el, 0) : null;
	};

	return {
		url: location.href,
		priceBlock: pick('#article-container > div'),
		articleHeader: pick('#article-container'),
		iconFeatures: pick('#section-icon-features-property'),
		generalFeatures: pick('#reactGeneralFeatures'),
		mapSection: pick('#map-section'),
		publisher: pick('#reactPublisherData'),
		agencyLead: (document.querySelector('[data-qa="linkMicrositioAnuncianteLeads"], [data-qa="linkMicrositioAnunciante"]') || {}).innerText || '',
		longDescription: (document.querySelector('#longDescription') || {}).innerText || '',
	};
}`

func probe(targetURL string) error {
	browser, l, err := startBrowser()
	if err != nil {
		return err
	}
	defer l.Kill()
	defer browser.Close()

	page, err := newPage(browser)
	if err != nil {
		return err
	}
	defer page.Close()

	if err := navigate(page, targetURL); err != nil {
		return err
	}

	result, err := page.Eval(probeScript)
	if err != nil {
		return err
	}

	var dump any
	if err := result.Value.Unmarshal(&dump); err != nil {
		return err
	}

	file, err := os.Create("probe.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(dump); err != nil {
		return err
	}

	fmt.Println("wrote probe.json")
	return nil
}
