package grading

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"golang.org/x/net/html"
)

type serverQuestion func(string, string) (bool, string, error)
type responseTester func(*http.Response) (bool, error)

func statusText(pass bool) string {
	if pass {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

// TestAll ...
func TestAll(rawURL string, showOutput bool) (int, int, error) {
	doLog := func(args ...interface{}) {
		if showOutput {
			fmt.Println(args...)
		}
	}
	numPass := 0
	numFail := 0
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return numPass, numFail, err
	}

	questions := []serverQuestion{
		indexIsUp,
		movieList,
		movieSearch,
		movieDetail,
	}
	for _, question := range questions {
		passed, questionText, err2 := question(parsedURL.Scheme, parsedURL.Host)
		doLog(statusText(passed && (err2 == nil)), "-", questionText)
		if passed {
			numPass++
		} else {
			numFail++
		}
	}
	return numPass, numFail, err
}

func newClient() *http.Client {
	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}
	return netClient
}

func testStatusEquals(response *http.Response, err error, questionText string, expectedStatus int) (bool, string, error) {
	if err != nil {
		return false, questionText, err
	}
	if response.StatusCode == expectedStatus {
		return true, questionText, nil
	}
	return false, questionText, nil
}

func readResponseBody(response *http.Response) (string, error) {
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)
	return bodyString, err
}

func testBodyEquals(response *http.Response, err error, questionText string, expectedBody string) (bool, string, error) {
	if err != nil {
		return false, questionText, err
	}
	dump, err2 := readResponseBody(response)
	if err2 != nil {
		return false, questionText, err
	}
	body := strings.Trim(string(dump), " ")
	if body == expectedBody {
		return true, questionText, nil
	}
	return false, questionText, nil
}

func testResponse(response *http.Response, err error, questionText string, testFunc responseTester) (bool, string, error) {
	if err != nil {
		return false, questionText, nil
	}
	result, err := testFunc(response)
	if result && err == nil {
		return true, questionText, nil
	}
	return false, questionText, nil
}

func getAndCheckFunction(scheme string, host string, urlPath string, query url.Values, questionText string, testFunc responseTester) (bool, string, error) {
	parsedURL := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     urlPath,
		RawQuery: query.Encode(),
	}
	netClient := newClient()
	response, err := netClient.Get(parsedURL.String())
	return testResponse(response, err, questionText, testFunc)
}

func getAndCheckStatus(scheme string, host string, urlPath string, query url.Values, questionText string, expectedStatus int) (bool, string, error) {
	parsedURL := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     urlPath,
		RawQuery: query.Encode(),
	}
	netClient := newClient()
	response, err := netClient.Get(parsedURL.String())
	return testStatusEquals(response, err, questionText, expectedStatus)
}

func getAndCheckBody(scheme string, host string, urlPath string, query url.Values, questionText string, expectedBody string) (bool, string, error) {
	testFunc := func(response *http.Response) (bool, error) {
		body, err := readResponseBody(response)
		if err != nil {
			return false, err
		}
		if body == expectedBody {
			return true, nil
		}
		return false, nil
	}
	return getAndCheckFunction(
		scheme,
		host,
		urlPath,
		query,
		questionText,
		testFunc,
	)
}

func indexIsUp(scheme string, baseURL string) (bool, string, error) {
	return getAndCheckStatus(
		scheme,
		baseURL,
		"/movies",
		url.Values{},
		"Your return a 200 status code at /movies",
		http.StatusOK,
	)
}

func debugHTML(n *html.Node) {
	var buf bytes.Buffer
	if err := html.Render(&buf, n); err != nil {
		log.Fatalf("Render error: %s", err)
	}
	fmt.Println(buf.String())
}

func getMovieCountChecker(numberOfMoviesExpected int) func(*http.Response) (bool, error) {
	hasMovies := func(response *http.Response) (bool, error) {
		doc, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			return false, err
		}
		elCount := doc.Find("tr.movie").Length()
		if elCount == numberOfMoviesExpected {
			return true, nil
		}
		return false, nil
	}
	return hasMovies
}

func movieDetail(scheme string, baseURL string) (bool, string, error) {
	orderedMovies := []string{"Spider-Man 2",
		"Titanic",
		"Troy",
		"Terminator 3: Rise of the Machines",
		"Waterworld",
		"Wild Wild West",
		"Van Helsing",
		"Alexander",
		"Master and Commander: The Far Side of the World",
		"Polar Express, The",
		"Tarzan",
		"Die Another Day"}
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s) // initialize local pseudorandom generator
	idx := r.Intn(len(orderedMovies))

	hasMovieTitle := func(response *http.Response) (bool, error) {
		doc, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			return false, err
		}
		selector := fmt.Sprintf("h1")
		if strings.Contains(doc.Find(selector).Text(), orderedMovies[idx]) {
			return true, nil
		}
		return false, nil
	}

	return getAndCheckFunction(
		scheme,
		baseURL,
		fmt.Sprintf("/movies/%d", idx+1),
		url.Values{},
		fmt.Sprintf("The URL /movies/%d has an h1 with content \"%s\"", idx, orderedMovies[idx]),
		hasMovieTitle,
	)
}

func movieList(scheme string, baseURL string) (bool, string, error) {
	numExpected := 499

	return getAndCheckFunction(
		scheme,
		baseURL,
		"/movies",
		url.Values{},
		fmt.Sprintf("There are %d movies (tr.movie) at /movies", numExpected),
		getMovieCountChecker(numExpected),
	)
}

func movieSearch(scheme string, baseURL string) (bool, string, error) {
	type ps struct {
		titleQuery    string
		expectedCount int
	}
	possibleSearches := []ps{
		ps{"Spider", 2},
		ps{"The", 148},
		ps{"last", 5},
		ps{"little", 5},
		ps{"no", 9},
		ps{"two", 3},
		ps{"III", 4},
	}
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s) // initialize local pseudorandom generator
	idx := r.Intn(len(possibleSearches))
	selected := possibleSearches[idx]
	query := url.Values{}

	query.Set("title", selected.titleQuery)

	return getAndCheckFunction(
		scheme,
		baseURL,
		fmt.Sprintf("/movies"),
		query,
		fmt.Sprintf("There are %d movies when searching for \"%s\" (no quotes)",
			selected.expectedCount, selected.titleQuery),
		getMovieCountChecker(selected.expectedCount),
	)
}
