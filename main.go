package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

type TTSRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
}

const randomArticleURL = "https://en.wikipedia.org/w/api.php?action=query&list=random&rnnamespace=0&rnlimit=1&rnminsize=5000&rnmaxsize=50000&format=json"

func tts(text string) error {
	godotenv.Load()
	elevenLabsKey := os.Getenv("ELEVENLABS_API_KEY")
	if elevenLabsKey == "" {
		log.Fatal("Eleven Labs key not set")
	}
	voiceID := "wBXNqKUATyqu0RtYt25i"

	url := "https://api.elevenlabs.io/v1/text-to-speech/" + voiceID

	reqBody := TTSRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("Marshalling error: %v\n", err)
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		return err
	}

	req.Header.Set("xi-api-key", elevenLabsKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("client error: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	fmt.Println("Sent to Eleven Labs")

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("IO read all error: %v\n", err)
		return err
	}

	if err := os.WriteFile("tts.mp3", audio, 0644); err != nil {
		fmt.Printf("error saving file: %v\n", err)
		return err
	}
	fmt.Println("tts saved")

	cmd := exec.Command("afplay", "tts.mp3")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error playing audio: %v\n", err)
		return err
	}

	return nil
}

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
		log.Fatal("OpenAI key not set")
	}

	random, err := wikiRequest(randomArticleURL, RA)
	if err != nil {
		fmt.Printf("wiki request error: %v\n", err)
		return
	}

	fmt.Printf("Random article found: %v\n", random.Query.Random[0].Title)

	title := strings.Split(random.Query.Random[0].Title, " ")
	newTitle := strings.Join(title, "_")

	url := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&prop=extracts&titles=%v&explaintext=1&format=json", newTitle)

	full, err := wikiRequest(url, FA)
	if err != nil {
		fmt.Printf("wiki request error: %v\n", err)
		return
	}

	fmt.Println("Information gathered")

	var page Page
	for _, p := range full.Query.Pages {
		page = p
		break
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	prompt := fmt.Sprintf(
		"State the name of this article, then provide a short paragraph explaining what it is, followed by another short paragraph with some fun facts:\n\n%s",
		page.Extract,
	)

	fmt.Println("Sending to OpenAI")

	resp, err := client.Responses.New(context.TODO(), responses.ResponseNewParams{
		Model: "gpt-5.6",
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(prompt)},
	})
	if err != nil {
		fmt.Printf("OpenAI error: %v", err)
	}

	fmt.Println("Processed by OpenAI")

	if err := tts(resp.OutputText()); err != nil {
		fmt.Printf("tts error: %v\n", err)
		return
	}
}
