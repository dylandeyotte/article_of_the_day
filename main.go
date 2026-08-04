package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type Article interface {
	FullArticle | RandomArticle
}

type RandomArticle struct {
	Batchcomplete string `json:"batchcomplete"`
	Continue      struct {
		Rncontinue string `json:"rncontinue"`
		Continue   string `json:"continue"`
	} `json:"continue"`
	Query struct {
		Random []struct {
			ID    int    `json:"id"`
			Ns    int    `json:"ns"`
			Title string `json:"title"`
		} `json:"random"`
	} `json:"query"`
}

type FullArticle struct {
	Query struct {
		Pages map[string]Page `json:"pages"`
	} `json:"query"`
}

type Page struct {
	PageID  int    `json:"pageid"`
	Ns      int    `json:"ns"`
	Title   string `json:"title"`
	Extract string `json:"extract"`
}

const randomArticleURL = "https://en.wikipedia.org/w/api.php?action=query&list=random&rnnamespace=0&rnlimit=1&rnminsize=5000&rnmaxsize=50000&format=json"

func wikiRequest[T Article](url string, article T) (T, error) {
	var zero T
	client := &http.Client{}

	req, err := http.NewRequest(
		"GET",
		url,
		nil,
	)
	if err != nil {
		return zero, err
	}

	req.Header.Set("User-Agent", "ArticleOfTheDay/1.0 (dylan091deyotte@gmail.com)")

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&article); err != nil {
		return zero, err
	}

	return article, nil
}

func main() {
	var RA RandomArticle
	var FA FullArticle

	godotenv.Load()
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("No key set")
	}

	random, err := wikiRequest(randomArticleURL, RA)
	if err != nil {
		fmt.Printf("wiki request error: %v\n", err)
		return
	}

	title := strings.Split(random.Query.Random[0].Title, " ")
	newTitle := strings.Join(title, "_")

	url := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&prop=extracts&titles=%v&explaintext=1&format=json", newTitle)

	full, err := wikiRequest(url, FA)
	if err != nil {
		fmt.Printf("wiki request error: %v\n", err)
		return
	}

	var page Page
	for _, p := range full.Query.Pages {
		page = p
		break
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	prompt := fmt.Sprintf(
		"Summarize this Wikipedia article:\n\n%s",
		page.Extract,
	)

	resp, err := client.Responses.New(context.TODO(), responses.ResponseNewParams{
		Model: "gpt-5.6",
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
	})
	if err != nil {
		fmt.Printf("OpenAI error: %v", err)
	}

	fmt.Println(resp.OutputText())
}
