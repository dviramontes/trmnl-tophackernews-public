package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheDir            = "cache"
	imageDir            = "headline_images_nano_banana"
	bestStoriesURL      = "https://hacker-news.firebaseio.com/v0/beststories.json"
	storyBaseURL        = "https://hacker-news.firebaseio.com/v0/item/"
	storiesToFetch      = 5
	imagePromptTemplate = "%s showcased in a gritty noir comic book splash page. High contrast chiaroscuro lighting, heavy ink lines, dramatic angle. Full bleed, edge-to-edge artwork, masterpiece."
	defaultImage        = "default.png"
	geminiModel         = "gemini-2.5-flash-image"
)

type Story struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Time        int64  `json:"time"`
	Descendants int    `json:"descendants"`
}

type FormattedStory struct {
	StoryTitle     string `json:"storyTitle"`
	StoryURL       string `json:"storyUrl"`
	StoryImage     string `json:"storyImage"`
	StoryTimestamp string `json:"storyTimestamp"`
	StoryID        int    `json:"storyId"`
	StoryScore     int    `json:"storyScore"`
}

type Response struct {
	Stories  []FormattedStory `json:"stories"`
	Metadata Metadata         `json:"metadata"`
}

type Metadata struct {
	TotalCount  int    `json:"totalCount"`
	LastUpdated string `json:"lastUpdated"`
	Version     string `json:"version"`
}

// type WebhookData struct {
// 	MergeVariables Response `json:"merge_variables"`
// }

type GeminiRequest struct {
	Contents         []GeminiContent  `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	ResponseModalities []string     `json:"responseModalities"`
	ImageConfig        *ImageConfig `json:"imageConfig,omitempty"`
}

type ImageConfig struct {
	AspectRatio string `json:"aspectRatio"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData *struct {
					Data     string `json:"data"`
					MimeType string `json:"mimeType"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type HackerNewsFeed struct {
	geminiAPIKey         string
	webhookURL           string
	forceUpdate          bool
	testMode             bool
	bestStoriesCacheFile string
}

func NewHackerNewsFeed() *HackerNewsFeed {
	feed := &HackerNewsFeed{
		geminiAPIKey:         os.Getenv("GEMINI_API_KEY"),
		webhookURL:           os.Getenv("TRMNL_WEBHOOK_URL"),
		forceUpdate:          os.Getenv("FORCE_UPDATE") == "true",
		testMode:             os.Getenv("TEST_MODE") == "true",
		bestStoriesCacheFile: filepath.Join(cacheDir, "beststories.json"),
	}

	feed.ensureDirectories()
	return feed
}

func (h *HackerNewsFeed) ensureDirectories() {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("Error creating cache directory: %v", err)
	}
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Printf("Error creating image directory: %v", err)
	}
}

func (h *HackerNewsFeed) getFeed(url, cacheFile string) ([]byte, error) {
	if !h.forceUpdate {
		if data, err := os.ReadFile(cacheFile); err == nil {
			return data, nil
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		// Try to fall back to cache
		if data, cacheErr := os.ReadFile(cacheFile); cacheErr == nil {
			return data, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		log.Printf("Error writing cache file: %v", err)
	}

	return data, nil
}

func (h *HackerNewsFeed) getStory(id int) (*Story, error) {
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%d.json", id))
	url := fmt.Sprintf("%s%d.json", storyBaseURL, id)

	data, err := h.getFeed(url, cacheFile)
	if err != nil {
		return nil, err
	}

	var story Story
	if err := json.Unmarshal(data, &story); err != nil {
		return nil, err
	}

	return &story, nil
}

func (h *HackerNewsFeed) cleanOldImages() {
	files, err := filepath.Glob(filepath.Join(imageDir, "*.jpg"))
	if err != nil {
		return
	}

	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().Before(thirtyDaysAgo) {
			os.Remove(file)
		}
	}
}

func (h *HackerNewsFeed) generateImage(prompt string, cacheID int) string {
	h.cleanOldImages()

	imagePath := filepath.Join(imageDir, fmt.Sprintf("%d.jpg", cacheID))

	// Check if image already exists
	if _, err := os.Stat(imagePath); err == nil {
		return imagePath
	}

	// Test mode: generate a small test image without calling API
	if h.testMode {
		log.Printf("TEST_MODE: Creating test image at %s", imagePath)
		// 1x1 red pixel JPEG
		testJPEG := []byte{
			0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
			0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
			0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
			0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
			0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
			0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
			0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
			0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03,
			0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d,
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
			0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
			0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62, 0x72,
			0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45,
			0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
			0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75,
			0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3,
			0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
			0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9,
			0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
			0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4,
			0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01,
			0x00, 0x00, 0x3f, 0x00, 0xfb, 0xd5, 0xdb, 0x20, 0xa8, 0xf1, 0x7e, 0xe9,
			0xf3, 0x61, 0xa0, 0x7f, 0xff, 0xd9,
		}
		if err := os.WriteFile(imagePath, testJPEG, 0644); err != nil {
			log.Printf("TEST_MODE: Error writing test image: %v", err)
			return defaultImage
		}
		return imagePath
	}

	if h.geminiAPIKey == "" {
		log.Println("GEMINI_API_KEY not set, using default image")
		return defaultImage
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, h.geminiAPIKey)

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: GenerationConfig{
			ImageConfig: &ImageConfig{AspectRatio: "4:3"},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return defaultImage
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return defaultImage
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return defaultImage
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return defaultImage
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return defaultImage
	}

	// Extract base64 image data
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		for _, part := range geminiResp.Candidates[0].Content.Parts {
			if part.InlineData != nil {
				imageData := part.InlineData.Data
				decoded, err := base64.StdEncoding.DecodeString(imageData)
				if err != nil {
					log.Printf("Error decoding base64: %v", err)
					return defaultImage
				}

				if err := os.WriteFile(imagePath, decoded, 0644); err != nil {
					log.Printf("Error writing image: %v", err)
					return defaultImage
				}

				return imagePath
			}
		}
	}

	log.Printf("No image data in response. Raw response: %s", string(body))
	return defaultImage
}

func (h *HackerNewsFeed) formatStory(id int, story *Story) FormattedStory {
	timestamp := time.Unix(story.Time, 0).Format("Jan 2, 2006")

	storyURL := story.URL
	if storyURL == "" {
		storyURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", id)
	}

	imagePrompt := fmt.Sprintf(imagePromptTemplate, story.Title)
	imagePath := h.generateImage(imagePrompt, id)

	return FormattedStory{
		StoryTitle:     story.Title,
		StoryURL:       storyURL,
		StoryImage:     imagePath,
		StoryTimestamp: timestamp,
		StoryID:        id,
		StoryScore:     story.Score,
	}
}

func (h *HackerNewsFeed) Render() []FormattedStory {
	data, err := h.getFeed(bestStoriesURL, h.bestStoriesCacheFile)
	if err != nil {
		log.Printf("Failed to fetch best stories: %v", err)
		return []FormattedStory{}
	}

	var storyIDs []int
	if err := json.Unmarshal(data, &storyIDs); err != nil {
		log.Printf("Failed to parse story IDs: %v", err)
		return []FormattedStory{}
	}

	// Limit to configured number of stories
	if len(storyIDs) > storiesToFetch {
		storyIDs = storyIDs[:storiesToFetch]
	}

	var stories []FormattedStory
	for _, id := range storyIDs {
		story, err := h.getStory(id)
		if err != nil {
			log.Printf("Failed to fetch story %d: %v", id, err)
			continue
		}
		stories = append(stories, h.formatStory(id, story))
	}

	return stories
}

// Commented out: TRMNL webhook publishing
// func (h *HackerNewsFeed) publishToTRMNL(data WebhookData) error {
// 	if h.webhookURL == "" {
// 		return fmt.Errorf("TRMNL_WEBHOOK_URL not set")
// 	}
//
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}
//
// 	req, err := http.NewRequest("POST", h.webhookURL, bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return err
// 	}
//
// 	req.Header.Set("Content-Type", "application/json")
//
// 	client := &http.Client{Timeout: 30 * time.Second}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()
//
// 	body, _ := io.ReadAll(resp.Body)
// 	log.Printf("Webhook Response Code: %d", resp.StatusCode)
// 	log.Printf("Webhook Response: %s", string(body))
//
// 	return nil
// }

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	feed := NewHackerNewsFeed()
	stories := feed.Render()

	response := Response{
		Stories: stories,
		Metadata: Metadata{
			TotalCount:  len(stories),
			LastUpdated: time.Now().Format(time.RFC3339),
			Version:     "1.0",
		},
	}

	// Output as JSON
	output, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal response: %v", err)
	}

	fmt.Println(string(output))

	// Commented out: TRMNL webhook publishing
	// webhookData := WebhookData{
	// 	MergeVariables: response,
	// }
	//
	// if err := feed.publishToTRMNL(webhookData); err != nil {
	// 	log.Printf("Failed to publish to TRMNL: %v", err)
	// }
}
