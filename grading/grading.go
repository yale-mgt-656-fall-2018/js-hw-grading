package grading

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type jsCodingSiteInfo struct {
	name       string
	domains    []string
	gradingFuc func(string) (float64, string, error)
}

var jsCodingSites = []jsCodingSiteInfo{
	{"FreeCodeCamp", []string{"www.freecodecamp.org"}, gradeFreeCodeCampProfile},
	{"CodeAcademy", []string{"www.codecademy.com"}, gradeCodeAcademyProfile},
}

type responseTester func(*http.Response) (bool, error)

func statusText(pass bool) string {
	if pass {
		return "✅"
	}
	return "❌"
}

// TestAll ...
func TestAll(rawURL string, showOutput bool) (int, int, error) {
	doLog := func(args ...interface{}) {
		if showOutput {
			fmt.Println(args...)
		}
	}
	maxScore := 60
	numPass := 0
	numFail := maxScore
	incrementScore := func(val int) {
		numPass += val
		numFail -= val
		if numFail < 0 {
			numFail = 0
		}
		if numPass > maxScore {
			numPass = maxScore
		}
	}

	// Give a few points if the URL submitted is valid and the site is up.
	//
	profileOKMaxScore := 5
	isValidSite, site, err := urlIsValidProfile(rawURL)
	isOnline := false
	if isValidSite && err == nil {
		isOnline, err = profileIsUp(rawURL)
	}
	profileOK := isOnline && isValidSite && err == nil
	profileOKscore := 0
	if profileOK {
		profileOKscore = 5
	}
	profileIsValidAndOnlineMsg := fmt.Sprintf("Profile is valid and online at %s (%d/%d pts)", rawURL, profileOKscore, profileOKMaxScore)
	doLog(statusText(profileOK), "-", profileIsValidAndOnlineMsg)
	if !profileOK {
		return numPass, numFail, nil
	}
	incrementScore(profileOKscore)

	// Score the assignment
	//
	fractionCompleted, fractionCompletedMsg, err := site.gradingFuc(rawURL)
	points := int(math.Round((fractionCompleted * float64(numFail))))
	emoji := statusText(false)
	if fractionCompleted == 1.0 {
		emoji = statusText(true)
	} else if fractionCompleted > 0 {
		emoji = "🔶"
	}
	fractionCompletedMsg += fmt.Sprintf(" (%d/%d pts)", points, numFail)
	doLog(emoji, "-", fractionCompletedMsg)
	incrementScore(points)
	return numPass, numFail, nil
}

func newClient() *http.Client {
	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}
	return netClient
}

func testStatusEquals(response *http.Response, err error, expectedStatus int) (bool, error) {
	if err != nil {
		return false, err
	}
	if response.StatusCode == expectedStatus {
		return true, nil
	}
	return false, nil
}

func readResponseBody(response *http.Response) (string, error) {
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)
	return bodyString, err
}

func testBodyEquals(response *http.Response, err error, expectedBody string) (bool, error) {
	if err != nil {
		return false, err
	}
	dump, err2 := readResponseBody(response)
	if err2 != nil {
		return false, err
	}
	body := strings.Trim(string(dump), " ")
	if body == expectedBody {
		return true, nil
	}
	return false, nil
}

func testResponse(response *http.Response, err error, testFunc responseTester) (bool, error) {
	if err != nil {
		return false, nil
	}
	result, err := testFunc(response)
	if result && err == nil {
		return true, nil
	}
	return false, nil
}

func getAndCheckFunction(scheme string, host string, urlPath string, query url.Values, testFunc responseTester) (bool, error) {
	parsedURL := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     urlPath,
		RawQuery: query.Encode(),
	}
	return getAndCheckFunctionForURL(parsedURL.String(), testFunc)
}

func getAndCheckFunctionForURL(theURL string, testFunc responseTester) (bool, error) {
	netClient := newClient()
	response, err := netClient.Get(theURL)
	return testResponse(response, err, testFunc)
}

func fetch(theURL string) (string, error) {
	netClient := newClient()
	response, err := netClient.Get(theURL)
	if err != nil {
		return "", err
	}
	return readResponseBody(response)
}

func getAndCheckStatusForURL(theURL string, expectedStatus int) (bool, error) {
	netClient := newClient()
	response, err := netClient.Get(theURL)
	return testStatusEquals(response, err, expectedStatus)
}

func getAndCheckStatus(scheme string, host string, urlPath string, query url.Values, expectedStatus int) (bool, error) {
	parsedURL := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     urlPath,
		RawQuery: query.Encode(),
	}
	return getAndCheckStatusForURL(parsedURL.String(), expectedStatus)
}

func getAndCheckBody(scheme string, host string, urlPath string, query url.Values, expectedBody string) (bool, error) {
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
		testFunc,
	)
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func urlIsValidProfile(theURL string) (bool, jsCodingSiteInfo, error) {
	parsedURL, err := url.Parse(theURL)
	if err != nil {
		return false, jsCodingSiteInfo{}, err
	}
	for _, site := range jsCodingSites {
		if stringSliceContains(site.domains, parsedURL.Host) {
			return true, site, nil
		}
	}
	return false, jsCodingSiteInfo{}, nil
}

func profileIsUp(theURL string) (bool, error) {
	return getAndCheckStatusForURL(
		theURL,
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

func gradeCodeAcademyProfile(profileURL string) (float64, string, error) {
	doReturn := func(credit float64, err error) (float64, string, error) {
		msg := fmt.Sprintf("You should have completed code academy")
		return credit, msg, err
	}
	netClient := newClient()
	response, err := netClient.Get(profileURL)
	if err != nil {
		log.Println(err)
		return doReturn(0, err)
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Println(err)
		return doReturn(0, errors.New("Error parsing your document"))
	}
	selector := "article#completed a[href*=\"introduction-to-javascript\"]"
	if doc.Find(selector).Length() > 0 {
		return doReturn(1., nil)
	}
	return doReturn(0., nil)
}

// The Basic JavaScript challenges are here:
// https://github.com/P1xt/fcc-is-forked/blob/master/src/app/services/challenges/data/02-javascript-algorithms-and-data-structures/basic-javascript.json

var freeCodeCampBasicJSChallengeIDS = []string{
	"bd7123c9c441eddfaeb4bdef",
	"bd7123c9c443eddfaeb5bdef",
	"56533eb9ac21ba0edf2244a8",
	"56533eb9ac21ba0edf2244a9",
	"56533eb9ac21ba0edf2244aa",
	"56533eb9ac21ba0edf2244ab",
	"cf1111c1c11feddfaeb3bdef",
	"cf1111c1c11feddfaeb4bdef",
	"cf1231c1c11feddfaeb5bdef",
	"cf1111c1c11feddfaeb6bdef",
	"56533eb9ac21ba0edf2244ac",
	"56533eb9ac21ba0edf2244ad",
	"cf1391c1c11feddfaeb4bdef",
	"bd7993c9c69feddfaeb7bdef",
	"bd7993c9ca9feddfaeb7bdef",
	"56533eb9ac21ba0edf2244ae",
	"56533eb9ac21ba0edf2244af",
	"56533eb9ac21ba0edf2244b0",
	"56533eb9ac21ba0edf2244b1",
	"56533eb9ac21ba0edf2244b2",
	"bd7123c9c444eddfaeb5bdef",
	"56533eb9ac21ba0edf2244b5",
	"56533eb9ac21ba0edf2244b4",
	"56533eb9ac21ba0edf2244b6",
	"56533eb9ac21ba0edf2244b7",
	"56533eb9ac21ba0edf2244b8",
	"56533eb9ac21ba0edf2244b9",
	"56533eb9ac21ba0edf2244ed",
	"bd7123c9c448eddfaeb5bdef",
	"bd7123c9c549eddfaeb5bdef",
	"56533eb9ac21ba0edf2244ba",
	"bd7123c9c450eddfaeb5bdef",
	"bd7123c9c451eddfaeb5bdef",
	"bd7123c9c452eddfaeb5bdef",
	"56533eb9ac21ba0edf2244bb",
	"bd7993c9c69feddfaeb8bdef",
	"cf1111c1c11feddfaeb7bdef",
	"56bbb991ad1ed5201cd392ca",
	"cf1111c1c11feddfaeb8bdef",
	"56592a60ddddeae28f7aa8e1",
	"56bbb991ad1ed5201cd392cb",
	"56bbb991ad1ed5201cd392cc",
	"56bbb991ad1ed5201cd392cd",
	"56bbb991ad1ed5201cd392ce",
	"56533eb9ac21ba0edf2244bc",
	"56bbb991ad1ed5201cd392cf",
	"56533eb9ac21ba0edf2244bd",
	"56533eb9ac21ba0edf2244be",
	"56533eb9ac21ba0edf2244bf",
	"56533eb9ac21ba0edf2244c0",
	"56533eb9ac21ba0edf2244c2",
	"598e8944f009e646fc236146",
	"56533eb9ac21ba0edf2244c3",
	"56533eb9ac21ba0edf2244c6",
	"bd7123c9c441eddfaeb5bdef",
	"cf1111c1c12feddfaeb3bdef",
	"56533eb9ac21ba0edf2244d0",
	"56533eb9ac21ba0edf2244d1",
	"599a789b454f2bbd91a3ff4d",
	"56533eb9ac21ba0edf2244d2",
	"56533eb9ac21ba0edf2244d3",
	"56533eb9ac21ba0edf2244d4",
	"56533eb9ac21ba0edf2244d5",
	"56533eb9ac21ba0edf2244d6",
	"56533eb9ac21ba0edf2244d7",
	"56533eb9ac21ba0edf2244d8",
	"56533eb9ac21ba0edf2244d9",
	"56533eb9ac21ba0edf2244da",
	"56533eb9ac21ba0edf2244db",
	"5690307fddb111c6084545d7",
	"56533eb9ac21ba0edf2244dc",
	"5664820f61c48e80c9fa476c",
	"56533eb9ac21ba0edf2244dd",
	"56533eb9ac21ba0edf2244de",
	"56533eb9ac21ba0edf2244df",
	"56533eb9ac21ba0edf2244e0",
	"5679ceb97cbaa8c51670a16b",
	"56533eb9ac21ba0edf2244c4",
	"565bbe00e9cc8ac0725390f4",
	"56bbb991ad1ed5201cd392d0",
	"56533eb9ac21ba0edf2244c7",
	"56533eb9ac21ba0edf2244c8",
	"56533eb9ac21ba0edf2244c9",
	"56bbb991ad1ed5201cd392d1",
	"56bbb991ad1ed5201cd392d2",
	"56bbb991ad1ed5201cd392d3",
	"56533eb9ac21ba0edf2244ca",
	"567af2437cbaa8c51670a16c",
	"56533eb9ac21ba0edf2244cb",
	"56533eb9ac21ba0edf2244cc",
	"56533eb9ac21ba0edf2244cd",
	"56533eb9ac21ba0edf2244cf",
	"cf1111c1c11feddfaeb1bdef",
	"cf1111c1c11feddfaeb5bdef",
	"56104e9e514f539506016a5c",
	"56105e7b514f539506016a5e",
	"5675e877dbd60be8ad28edc6",
	"56533eb9ac21ba0edf2244e1",
	"5a2efd662fb457916e1fe604",
	"5688e62ea601b2482ff8422b",
	"cf1111c1c11feddfaeb9bdef",
	"cf1111c1c12feddfaeb1bdef",
	"cf1111c1c12feddfaeb2bdef",
	"587d7b7e367417b2b2512b23",
	"587d7b7e367417b2b2512b22",
	"587d7b7e367417b2b2512b24",
	"587d7b7e367417b2b2512b21",
}
var freeCodeCampReactChallengeIDS = []string{
	"587d7dbc367417b2b2512bb1",
	"5a24bbe0dba28a8d3cbd4c5d",
	"5a24bbe0dba28a8d3cbd4c5e",
	"5a24bbe0dba28a8d3cbd4c5f",
	"5a24c314108439a4d4036160",
	"5a24c314108439a4d4036161",
	"5a24c314108439a4d4036162",
	"5a24c314108439a4d4036163",
	"5a24c314108439a4d4036164",
	"5a24c314108439a4d4036165",
	"5a24c314108439a4d4036166",
	"5a24c314108439a4d4036167",
	"5a24c314108439a4d4036168",
	"5a24c314108439a4d4036169",
	"5a24c314108439a4d403616a",
	"5a24c314108439a4d403616b",
	"5a24c314108439a4d403616c",
	"5a24c314108439a4d403616d",
	"5a24c314108439a4d403616e",
	"5a24c314108439a4d403616f",
	"5a24c314108439a4d4036170",
	"5a24c314108439a4d4036171",
	"5a24c314108439a4d4036172",
	"5a24c314108439a4d4036173",
	"5a24c314108439a4d4036174",
	"5a24c314108439a4d4036176",
	"5a24c314108439a4d4036177",
	"5a24c314108439a4d4036178",
	"5a24c314108439a4d4036179",
	"5a24c314108439a4d403617a",
	"5a24c314108439a4d403617b",
	"5a24c314108439a4d403617c",
	"5a24c314108439a4d403617d",
	"5a24c314108439a4d403617e",
	"5a24c314108439a4d403617f",
	"5a24c314108439a4d4036180",
	"5a24c314108439a4d4036181",
	"5a24c314108439a4d4036182",
	"5a24c314108439a4d4036183",
	"5a24c314108439a4d4036184",
	"5a24c314108439a4d4036185",
	"5a24c314108439a4d4036187",
	"5a24c314108439a4d4036188",
	"5a24c314108439a4d4036189",
	"5a24c314108439a4d403618a",
	"5a24c314108439a4d403618b",
	"5a24c314108439a4d403618c",
	"5a24c314108439a4d403618d",
}

func countChallengesCompleted(body string, challenges []string) int {
	challengesCompleted := 0
	for _, challengeID := range challenges {
		if strings.Contains(body, challengeID) {
			challengesCompleted++
		}
	}
	return challengesCompleted
}

func gradeFreeCodeCampProfile(profileURL string) (float64, string, error) {
	msg := fmt.Sprintf("You completed freecodecamp Basic JavaScript or React")
	doReturn := func(credit float64, err error) (float64, string, error) {
		return credit, msg, err
	}

	parsedURL, err := url.Parse(profileURL)
	if err != nil {
		return doReturn(0, err)
	}
	userID := strings.Trim(parsedURL.Path, "/")
	apiURL := "https://www.freecodecamp.org/api/users/get-public-profile?username=" + userID

	body, err := fetch(apiURL)
	maxScore := 0.
	challengeSets := [][]string{freeCodeCampBasicJSChallengeIDS, freeCodeCampReactChallengeIDS}
	for _, challengeSet := range challengeSets {
		challengesCompleted := countChallengesCompleted(body, challengeSet)
		score := float64(challengesCompleted) / float64(len(challengeSet))
		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore, msg, nil
}
