package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	URLlist    = flag.String("url", "", "Path to list of URLs to test.")
	ProxyToUse = flag.String("proxy", "", "Specify proxy if desired.")
	UseProxy   = false
)

// initialize seed.  Next several lines stolen from SO post where @icza pulls random strings all together
// Muchas gracias!
func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandString(n int) string {
	myrune := make([]rune, n)
	for i := range myrune {
		myrune[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(myrune)
}

// Just getting the urls line by line out of list.
func getDomains(path *string) (lines []string, Error error) {
	file, err := os.Open(*path)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

func makeRequest(mutants []string, url string) { //(myresponse *http.Response) {
	client := &http.Client{}
	if UseProxy == true {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyFromEnvironment,
		}
		client = &http.Client{Transport: tr}
	}

	for _, mutant := range mutants {
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36")
		req.Header.Add("Origin", mutant)
		if err != nil {
			log.Fatalln(err)
		}

		response, err := client.Do(req)
		if err != nil {
			log.Fatalln(err)
		}
		headersmap := response.Header
		found := responseCheck1(mutant, strings.Join(headersmap["Access-Control-Allow-Origin"][:], " "))
		if found {
			fmt.Println("Eureka! For ", url, " ", mutant, " was in ", headersmap["Access-Control-Allow-Origin"][:])
			found := responseCheck2(strings.Join(headersmap["Access-Control-Allow-Credentials"][:], " "))
			if found {
				fmt.Println("Eureka! For ", url, " ", mutant, " also has ACAC: true!")
			} else {
				fmt.Println("No ACAC")
			}
		} else {
			fmt.Println("No match for", url, ":", mutant)
		}

		//return response
	}
}

func responseCheck1(mutant string, resporigin string) (check bool) {
	r, err := regexp.Compile(mutant)
	if err != nil {
		fmt.Println("Something went wrong.  Generated origin not a valid regex. ", mutant)
	}
	checkresult := r.MatchString(resporigin)
	return checkresult
}

func responseCheck2(respacac string) (check bool) {
	r, err := regexp.Compile("true")
	if err != nil {
		fmt.Println("Something went wrong. :(", respacac)
	}
	checkresult := r.MatchString(respacac)
	return checkresult
}

//  Create some versions of commonly accepted dynamic CORS policies.
func mutateOrigin(targeturl string) (mutants []string) {
	parsdUrl, err := url.Parse(targeturl)
	if err != nil {
		fmt.Println("Failed to parse target: " + targeturl)
	}
	// Test to see if it will return Access-Control-Allow-Origin of any old thing.
	mutants = append(mutants, parsdUrl.Scheme+"://"+RandString(12)+".com")
	// Test <something>target
	r, err := regexp.Compile(`\w+[.]\w+$`)
	targetdomain := r.FindString(parsdUrl.Host)
	mutants = append(mutants, parsdUrl.Scheme+"://"+RandString(12)+targetdomain)
	// Test <something as subdomain>.target
	mutants = append(mutants, parsdUrl.Scheme+"://"+RandString(5)+"."+targetdomain)
	// Test baseurl.<something after>
	mutants = append(mutants, parsdUrl.Scheme+"://"+targetdomain+"."+RandString(12)+".com")
	// Test for the null origin
	mutants = append(mutants, "null")
	return mutants
}

func main() {
	// execution timer.  Just for playing around.
	start := time.Now()

	var wg sync.WaitGroup
	// Handle flags
	flag.Parse()

	if flag.NFlag() == 0 {
		flag.Usage()
		os.Exit(0)
	}

	flagset := make(map[string]bool)
	// TODO: Look into using Cobra pkg and Changed() to clean up.
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })

	if flagset["proxy"] {
		os.Setenv("HTTPS_PROXY", *ProxyToUse)
		fmt.Println("Set user specified proxy " + *ProxyToUse)
		UseProxy = true
	}

	urls, err := getDomains(URLlist)
	if err != nil {
		log.Fatalln(err)
	}

	wg.Add(len(urls))
	for _, targeturl := range urls {
		// Things we want to do with each base target
		mutants := mutateOrigin(targeturl)
		go func(targeturl string) {
			defer wg.Done()
			makeRequest(mutants, targeturl)
		}(targeturl)
	}

	elapsed := time.Since(start)
	wg.Wait()
	fmt.Printf("This took %s to execute.\n", elapsed)
}
