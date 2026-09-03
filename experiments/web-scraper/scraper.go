package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var (
	// perNeighborhood and outputPath are set from flags so a short run can
	// validate the extraction before committing to the full, human-gated pass.
	perNeighborhood = propertiesPerNeighborhood
	outputPath      = outputFile
)

const (
	propertiesPerNeighborhood = 100
	scrapeWorkers             = 2
	outputFile                = "../../data/listings.jsonl"
	pageTimeout               = 90 * time.Second
	// A challenge you have to solve by hand needs far longer than a page load.
	challengeTimeout = 5 * time.Minute
	// Cloudflare escalates when requests arrive back to back from one IP.
	requestDelay = 1500 * time.Millisecond
	// Persisting the profile keeps cf_clearance between runs, so the challenge
	// is solved once instead of on every start.
	profileDir = ".chrome-profile"
)

type Neighborhood struct {
	Name string
	URL  string
}

// Property is one scraped listing. It mirrors internal/listing.Raw field for
// field, and the JSON tags are the contract between the two: this scraper is a
// separate Go module by design, so the struct cannot be shared and the tags
// must be kept in step with internal/listing/raw.go by hand.
//
// Everything here is text exactly as the page renders it. Turning "$ 450.000"
// into an amount is deliberately left to internal/listing.Normalize, which is
// a pure function under test and can be corrected and re-run without visiting
// the site a second time.
type Property struct {
	Source       string            `json:"source"`
	URL          string            `json:"url"`
	Neighborhood string            `json:"neighborhood"`
	Agency       string            `json:"agency,omitempty"`
	Address      string            `json:"address,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description"`
	PriceText    string            `json:"price_text,omitempty"`
	ExpensesText string            `json:"expenses_text,omitempty"`
	Features     map[string]string `json:"features,omitempty"`
	ScrapedAt    time.Time         `json:"scraped_at"`
}

// extraction is the shape returned by extractScript.
type extraction struct {
	HasContainer bool              `json:"hasContainer"`
	Agency       string            `json:"agency"`
	Address      string            `json:"address"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	PriceText    string            `json:"priceText"`
	ExpensesText string            `json:"expensesText"`
	Features     map[string]string `json:"features"`
}

var neighborhoods = []Neighborhood{
	{Name: "congreso", URL: "https://www.zonaprop.com.ar/departamentos-alquiler-congreso.html"},
	{Name: "palermo", URL: "https://www.zonaprop.com.ar/departamentos-alquiler-palermo.html"},
	{Name: "monserrat", URL: "https://www.zonaprop.com.ar/departamentos-alquiler-monserrat.html"},
}

func startBrowser() (*rod.Browser, *launcher.Launcher, error) {
	browserPath, found := launcher.LookPath()
	if !found {
		return nil, nil, fmt.Errorf("Chrome or Chromium is not installed")
	}

	dir, err := filepath.Abs(profileDir)
	if err != nil {
		return nil, nil, err
	}

	l := launcher.New().
		Bin(browserPath).
		Headless(false).
		Leakless(false).
		UserDataDir(dir).
		// --enable-automation sets navigator.webdriver and flags the browser as
		// automated; this flag removes the signal at the browser level instead of
		// papering over it from JS.
		Delete("enable-automation").
		Set("disable-blink-features", "AutomationControlled").
		// Cloudflare's challenge widget lives in a cross-origin iframe, so leave
		// site isolation alone.
		Delete("disable-site-isolation-trials").
		Set("disable-features", "TranslateUI").
		Set("lang", "es-AR")

	controlURL, err := l.Launch()
	if err != nil {
		return nil, nil, err
	}

	// NoDefaultDevice is the important one: rod otherwise emulates a "laptop"
	// device that overrides the user agent to Chrome 114 and blanks out the
	// Sec-CH-UA client hints, which no real Chrome would ever do.
	browser := rod.New().ControlURL(controlURL).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		l.Kill()
		return nil, nil, err
	}
	return browser, l, nil
}

func newPage(browser *rod.Browser) (*rod.Page, error) {
	return browser.Page(proto.TargetCreateTarget{})
}

func navigate(page *rod.Page, targetURL string) error {
	nav := page.Timeout(pageTimeout)
	if err := nav.Navigate(targetURL); err != nil {
		nav.CancelTimeout()
		return err
	}
	if err := nav.WaitLoad(); err != nil {
		nav.CancelTimeout()
		return err
	}
	nav.CancelTimeout()

	return waitForChallenge(page, targetURL)
}

// waitForChallenge blocks while Cloudflare is interposing a challenge. It does
// not try to solve it; it waits for the real page to appear, which may mean the
// human at the keyboard clicks the checkbox.
func waitForChallenge(page *rod.Page, targetURL string) error {
	deadline := time.Now().Add(pageTimeout)
	announced := false

	for {
		challenged, err := onChallengePage(page)
		if err != nil {
			return err
		}
		if !challenged {
			return nil
		}

		if !announced {
			announced = true
			deadline = time.Now().Add(challengeTimeout)
			fmt.Printf("Cloudflare challenge on %s - solve it in the browser window; waiting up to %s\n",
				targetURL, challengeTimeout)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Cloudflare challenge did not finish for %s", targetURL)
		}
		time.Sleep(time.Second)
	}
}

func onChallengePage(page *rod.Page) (bool, error) {
	info, err := page.Info()
	if err != nil {
		return false, err
	}
	title := strings.ToLower(info.Title)
	if strings.Contains(title, "just a moment") ||
		strings.Contains(title, "un momento") ||
		strings.Contains(title, "attention required") ||
		strings.Contains(info.URL, "__cf_chl") {
		return true, nil
	}

	// Newer managed challenges keep the site's own title, so look for the
	// challenge markup as well.
	result, err := page.Eval(`() => Boolean(
		document.getElementById('challenge-form') ||
		document.getElementById('challenge-running') ||
		document.querySelector('#cf-challenge-running, .cf-browser-verification, #turnstile-wrapper') ||
		document.querySelector('iframe[src*="challenges.cloudflare.com"]')
	)`)
	if err != nil {
		return false, err
	}
	return result.Value.Bool(), nil
}

func collectProperties(browser *rod.Browser, neighborhood Neighborhood) ([]Property, error) {
	page, err := newPage(browser)
	if err != nil {
		return nil, err
	}
	defer page.Close()

	properties := make([]Property, 0, perNeighborhood)
	seen := make(map[string]bool)

	for pageNumber := 1; len(properties) < perNeighborhood; pageNumber++ {
		if pageNumber > 50 {
			return nil, fmt.Errorf("%s: only found %d properties", neighborhood.Name, len(properties))
		}

		if err := navigate(page, paginationURL(neighborhood.URL, pageNumber)); err != nil {
			return nil, fmt.Errorf("%s page %d: %w", neighborhood.Name, pageNumber, err)
		}

		hrefs, err := propertyLinks(page)
		if err != nil {
			return nil, fmt.Errorf("%s page %d: %w", neighborhood.Name, pageNumber, err)
		}
		previousCount := len(properties)

		for _, href := range hrefs {
			propertyURL, valid := canonicalPropertyURL(href)
			if !valid || seen[propertyURL] {
				continue
			}

			seen[propertyURL] = true
			properties = append(properties, Property{
				Neighborhood: neighborhood.Name,
				URL:          propertyURL,
			})
			if len(properties) == perNeighborhood {
				break
			}
		}

		if len(properties) == previousCount {
			return nil, fmt.Errorf("%s page %d contained no new properties", neighborhood.Name, pageNumber)
		}
	}

	return properties, nil
}

func propertyLinks(page *rod.Page) ([]string, error) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		result, err := page.Eval(`() => Array.from(document.querySelectorAll('a[href*="/propiedades/clasificado/"]'), link => link.href)`)
		if err != nil {
			return nil, err
		}

		var hrefs []string
		if err := result.Value.Unmarshal(&hrefs); err != nil {
			return nil, err
		}
		if len(hrefs) > 0 {
			return hrefs, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("property links were not found")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func canonicalPropertyURL(rawURL string) (string, bool) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Hostname() != "www.zonaprop.com.ar" && parsedURL.Hostname() != "zonaprop.com.ar") {
		return "", false
	}
	if !strings.Contains(parsedURL.Path, "/propiedades/clasificado/") || !strings.HasSuffix(parsedURL.Path, ".html") {
		return "", false
	}

	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String(), true
}

func paginationURL(baseURL string, page int) string {
	if page == 1 {
		return baseURL
	}
	return strings.TrimSuffix(baseURL, ".html") + fmt.Sprintf("-pagina-%d.html", page)
}

// scrapeAll visits every collected listing and writes each success to disk as
// it lands.
//
// A single unreadable listing does not abort the run. Three hundred pages
// behind a Cloudflare challenge is a long, human-gated operation, and losing
// all of it because one listing was withdrawn mid-scrape would be the wrong
// trade. Failures are counted and reported instead.
func scrapeAll(browser *rod.Browser, properties []Property, sink *jsonlWriter) (int, []error) {
	jobs := make(chan int)
	var workers sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	succeeded := 0

	for range scrapeWorkers {
		workers.Add(1)
		go func() {
			defer workers.Done()

			page, err := newPage(browser)
			if err != nil {
				mu.Lock()
				failures = append(failures, err)
				mu.Unlock()
				for range jobs {
				}
				return
			}
			defer page.Close()

			for index := range jobs {
				time.Sleep(requestDelay)

				if err := scrapeProperty(page, &properties[index]); err != nil {
					mu.Lock()
					failures = append(failures, fmt.Errorf("%s: %w", properties[index].URL, err))
					mu.Unlock()
					continue
				}

				mu.Lock()
				writeErr := sink.write(properties[index])
				if writeErr != nil {
					failures = append(failures, writeErr)
				} else {
					succeeded++
					if succeeded%25 == 0 {
						fmt.Printf("  scraped %d/%d\n", succeeded, len(properties))
					}
				}
				mu.Unlock()
			}
		}()
	}

	for index := range properties {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	return succeeded, failures
}

// jsonlWriter appends one listing per line, flushing as it goes so a run that
// dies partway through still leaves everything it had already read.
type jsonlWriter struct {
	file    *os.File
	encoder *json.Encoder
}

func newJSONLWriter(path string) (*jsonlWriter, error) {
	// The default output is ../../data/, which a fresh clone may not have yet.
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	return &jsonlWriter{file: file, encoder: encoder}, nil
}

func (w *jsonlWriter) write(property Property) error {
	if err := w.encoder.Encode(property); err != nil {
		return err
	}
	return w.file.Sync()
}

func (w *jsonlWriter) Close() error { return w.file.Close() }

// extractScript reads the listing fields off the rendered page.
//
// The anchors were chosen by probing a real listing (go run . -probe <url>).
// ZonaProp's layout class names are hashed and rotate between deploys, so the
// selectors deliberately hang off things that carry meaning instead: element
// ids, data-qa attributes, and the semantic icon-* class on each feature row.
const extractScript = `() => {
	const clean = s => (s || '').replace(/\s+/g, ' ').trim();
	const textOf = sel => {
		const el = document.querySelector(sel);
		return el ? clean(el.innerText) : '';
	};

	// Each feature is an <li> whose <i> carries a semantic class such as
	// icon-stotal or icon-ambiente; the readable value is on the <li>.
	const iconFeature = name => {
		const icon = document.querySelector('#section-icon-features-property i[class*="icon-' + name + '"]');
		return icon && icon.parentElement ? clean(icon.parentElement.innerText) : '';
	};

	// Keep the paragraph breaks: the description is what the parser reads, and
	// the structure helps it far more than it costs in bytes.
	const description = document.querySelector('#longDescription');

	return {
		hasContainer: document.getElementById('article-container') !== null,
		agency: textOf('[data-qa="linkMicrositioAnuncianteLeads"]') ||
			textOf('[data-qa="linkMicrositioAnunciante"]'),
		address: textOf('#map-section h4'),
		title: textOf('h1'),
		description: description ? description.innerText.trim() : '',
		priceText: textOf('#article-container .price-value span'),
		expensesText: textOf('#article-container .price-expenses'),
		features: {
			'superficie total': iconFeature('stotal'),
			'superficie cubierta': iconFeature('scubierta'),
			'ambientes': iconFeature('ambiente'),
			'dormitorios': iconFeature('dormitorio'),
			'banos': iconFeature('bano'),
			'cocheras': iconFeature('cochera'),
			'antiguedad': iconFeature('antiguedad'),
			'disposicion': iconFeature('disposicion'),
		},
	};
}`

func scrapeProperty(page *rod.Page, property *Property) error {
	if err := navigate(page, property.URL); err != nil {
		return err
	}

	result, err := page.Eval(extractScript)
	if err != nil {
		return err
	}

	var found extraction
	if err := result.Value.Unmarshal(&found); err != nil {
		return err
	}
	if !found.HasContainer {
		return fmt.Errorf("article-container was not found")
	}
	if found.Description == "" {
		return fmt.Errorf("listing has no description")
	}

	property.Source = "zonaprop"
	property.Agency = found.Agency
	property.Address = found.Address
	property.Title = found.Title
	property.Description = found.Description
	property.PriceText = found.PriceText
	property.ExpensesText = found.ExpensesText
	property.Features = dropEmpty(found.Features)
	property.ScrapedAt = time.Now().UTC()
	return nil
}

// dropEmpty removes fields the listing did not publish, so an absent value
// stays absent rather than becoming an empty string downstream.
func dropEmpty(features map[string]string) map[string]string {
	kept := make(map[string]string, len(features))
	for label, value := range features {
		if strings.TrimSpace(value) != "" {
			kept[label] = value
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func warmUp(browser *rod.Browser) error {
	page, err := newPage(browser)
	if err != nil {
		return err
	}
	defer page.Close()

	if err := navigate(page, neighborhoods[0].URL); err != nil {
		return err
	}
	fmt.Println("Cloudflare cleared, starting scrape")
	return nil
}

func run() error {
	browser, l, err := startBrowser()
	if err != nil {
		return err
	}
	defer l.Kill()
	defer browser.Close()

	// Clear the challenge once up front so the cf_clearance cookie is already in
	// the profile before any concurrent page starts loading.
	if err := warmUp(browser); err != nil {
		return err
	}

	properties := make([]Property, 0, len(neighborhoods)*perNeighborhood)
	for _, neighborhood := range neighborhoods {
		collected, err := collectProperties(browser, neighborhood)
		if err != nil {
			return err
		}
		fmt.Printf("collected %d listing urls in %s\n", len(collected), neighborhood.Name)
		properties = append(properties, collected...)
	}

	sink, err := newJSONLWriter(outputPath)
	if err != nil {
		return err
	}
	defer sink.Close()

	succeeded, failures := scrapeAll(browser, properties, sink)

	fmt.Printf("saved %d of %d listings to %s\n", succeeded, len(properties), outputPath)
	if len(failures) > 0 {
		fmt.Printf("%d listings failed:\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("  %v\n", failure)
		}
	}
	if succeeded == 0 {
		return fmt.Errorf("no listings were scraped")
	}
	return nil
}

func main() {
	probeURL := flag.String("probe", "", "dump one listing page's structure to probe.json and exit")
	limit := flag.Int("limit", propertiesPerNeighborhood, "listings to scrape per neighborhood")
	out := flag.String("out", outputFile, "path to write the scraped JSONL to")
	flag.Parse()

	perNeighborhood = *limit
	outputPath = *out

	if *probeURL != "" {
		if err := probe(*probeURL); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
