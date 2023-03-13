package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// Map for the crawler status
type State map[string]bool

func crawl(url, stateFile string, startURL string, destDir string) (State, error) {
	// Load the status
	state, err := loadState(stateFile)
	if err != nil {
		return state, err
	}
	err = processPage(url, state, startURL, destDir, stateFile)
	return state, err
}

func loadState(stateFile string) (map[string]bool, error) {
	state := make(map[string]bool)
	if _, err := os.Stat(stateFile); err == nil {
		file, err := os.Open(stateFile)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func saveState(state map[string]bool, stateFile string) error {
	file, err := os.Create(stateFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(state); err != nil {
		return err
	}
	return nil
}

// Recursive function to process the page
func processPage(urlStr string, state State, startURL string, destDir string, stateFile string) error {

	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to get URL %s: %v", urlStr, err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	defer resp.Body.Close()

	// Parse HTML content
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse HTML content: %v", err)
	}

	// Find all <a> tags and extract their href attributes
	var links []string
	var findLinks func(*html.Node)
	findLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findLinks(c)
		}
	}
	findLinks(doc)

	u, err := url.Parse(urlStr)
	if err != nil {
		fmt.Printf("failed to parse URL %s: %v", urlStr, err)
	}

	savePath := path.Join(destDir, u.Hostname(), u.Path)
	err = savePage(bodyBytes, savePath) //! TODO can be concurrent
	if err != nil {
		fmt.Printf("failed to download/save URL %s: %v", urlStr, err)
		return err
	}

	// Page visited
	state[urlStr] = true
	// Save the new state
	saveState(state, stateFile)

	// Filter valid URLs and download/save their content
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			fmt.Printf("failed to parse URL %s: %v", link, err)
			continue
		}
		if u.Host != "" && !strings.HasPrefix(urlStr, startURL) {
			fmt.Printf("Skip URLs with a different %s", link)
			continue
		}
		if path.Ext(u.Path) != ".html" {
			fmt.Printf("Skip non-HTML URLs %s %s\n", path.Ext(u.Path), link)
			continue
		}
		if _, ok := state[link]; !ok {
			err := processPage(link, state, startURL, destDir, stateFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func savePage(data []byte, savePath string) error {
	fmt.Print(savePath)
	path := filepath.Dir(savePath)
	os.MkdirAll(path, os.ModePerm)
	// Check if file exists
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		// File does not exist, create it
		file, err := os.Create(savePath)
		if err != nil {
			fmt.Println("Error creating file:", err)
			return err
		}
		defer file.Close()
		_, err = file.Write(data)
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return err
		}
		return nil
	} else {
		return errors.New("File already exists")
	}
}

func main() {
	stateFile := "state.json"

	startURL := flag.String("start", "", "Starting URL")
	destDir := flag.String("dir", "", "Destination directory")
	flag.Parse()

	if len(*startURL) == 0 || len(*destDir) == 0 {
		fmt.Print("use command -start <url> -dir <directory>\n")
	}

	state, err := crawl(*startURL, stateFile, *startURL, *destDir)
	if err != nil {
		fmt.Println("Errore durante il crawling:", err)
		return
	}

	// Print visited page
	fmt.Println("Visited page:")
	for url := range state {
		fmt.Println(url)
	}
}
