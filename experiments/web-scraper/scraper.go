package main

import (
	"encoding/json"
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

const (
	propertiesPerNeighborhood = 100
	scrapeWorkers             = 2
	outputFile                = "property_requirements.jsonl"
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

type Property struct {
	Neighborhood string `json:"neighborhood"`
	URL          string `json:"url"`
	Requirements string `json:"requirements"`
}

type sectionResult struct {
	HasContainer bool   `json:"hasContainer"`
	Text         string `json:"text"`
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

	properties := make([]Property, 0, propertiesPerNeighborhood)
	seen := make(map[string]bool)

	for pageNumber := 1; len(properties) < propertiesPerNeighborhood; pageNumber++ {
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
			if len(properties) == propertiesPerNeighborhood {
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

func scrapeRequirements(browser *rod.Browser, properties []Property) error {
	jobs := make(chan int)
	var workers sync.WaitGroup
	var errorLock sync.Mutex
	var firstError error

	for range scrapeWorkers {
		workers.Add(1)
		go func() {
			defer workers.Done()

			page, err := newPage(browser)
			if err != nil {
				errorLock.Lock()
				if firstError == nil {
					firstError = err
				}
				errorLock.Unlock()
				for range jobs {
				}
				return
			}
			defer page.Close()

			for index := range jobs {
				errorLock.Lock()
				stopped := firstError != nil
				errorLock.Unlock()
				if stopped {
					continue
				}

				time.Sleep(requestDelay)
				requirements, err := scrapeProperty(page, properties[index].URL)
				if err != nil {
					errorLock.Lock()
					if firstError == nil {
						firstError = fmt.Errorf("%s: %w", properties[index].URL, err)
					}
					errorLock.Unlock()
					continue
				}
				properties[index].Requirements = requirements
			}
		}()
	}

	for index := range properties {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	return firstError
}

func scrapeProperty(page *rod.Page, propertyURL string) (string, error) {
	if err := navigate(page, propertyURL); err != nil {
		return "", err
	}

	result, err := page.Eval(`() => {
		const container = document.getElementById('article-container');
		const section = document.evaluate('//*[@id="article-container"]/section[3]', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		return {
			hasContainer: container !== null,
			text: section ? section.innerText.replace(/\s+/g, ' ').trim() : ''
		};
	}`)
	if err != nil {
		return "", err
	}

	var section sectionResult
	if err := result.Value.Unmarshal(&section); err != nil {
		return "", err
	}
	if !section.HasContainer {
		return "", fmt.Errorf("article-container was not found")
	}
	return section.Text, nil
}

func writeProperties(properties []Property) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	for _, property := range properties {
		if err := encoder.Encode(property); err != nil {
			return err
		}
	}
	return nil
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

	properties := make([]Property, 0, len(neighborhoods)*propertiesPerNeighborhood)
	for _, neighborhood := range neighborhoods {
		collected, err := collectProperties(browser, neighborhood)
		if err != nil {
			return err
		}
		properties = append(properties, collected...)
	}

	if err := scrapeRequirements(browser, properties); err != nil {
		return err
	}
	if err := writeProperties(properties); err != nil {
		return err
	}

	fmt.Printf("saved %d properties to %s\n", len(properties), outputFile)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
