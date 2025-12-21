<?php
declare(strict_types=1);

// Error handling
ini_set('display_errors', '1');
ini_set('display_startup_errors', '1');
error_reporting(E_ALL);

class HackerNewsFeed
{
    // Change these
    private const GEMINI_API_KEY = 'YOUR_GEMINI_API_KEY';
    public const WEBHOOK_URL = 'YOUR_TRMNL_WEBHOOK_URL';
    // These should be fine
    private const CACHE_DIR = 'cache/';
    private const IMAGE_DIR_NANO_BANANA = 'headline_images_nano_banana/';
    private const BEST_STORIES_URL = 'https://hacker-news.firebaseio.com/v0/beststories.json';
    private const STORY_BASE_URL = 'https://hacker-news.firebaseio.com/v0/item/';
    private const STORIES_TO_FETCH = 5;
    private const IMAGE_PROMPT_TEMPLATE = '%s showcased in a gritty noir comic book splash page. High contrast chiaroscuro lighting, heavy ink lines, dramatic angle. Full bleed, edge-to-edge artwork, masterpiece.';
    private const DEFAULT_IMAGE = 'default.png';


    private string $bestStoriesCacheFile;
    private bool $forceUpdate;
    private string $remoteImageBaseUrl;

    public function __construct()
    {
        $this->bestStoriesCacheFile = self::CACHE_DIR . 'beststories.json';
        $this->forceUpdate = isset($_GET['update']) && $_GET['update'] === 'true';

        $protocol = isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] === 'on' ? "https" : "http";
        $host = $_SERVER['HTTP_HOST'] ?? 'localhost';
        $path = rtrim(dirname($_SERVER['PHP_SELF']), '/\\');
        $this->remoteImageBaseUrl = "$protocol://$host$path/";

        $this->ensureDirectories();
    }

    private function ensureDirectories(): void
    {
        if (!is_dir(self::CACHE_DIR)) {
            mkdir(self::CACHE_DIR, 0755, true);
        }
        if (!is_dir(self::IMAGE_DIR_NANO_BANANA)) {
            mkdir(self::IMAGE_DIR_NANO_BANANA, 0755, true);
        }
    }

    private function getFeed(string $url, string $cacheFile): ?string
    {
        if (!$this->forceUpdate && file_exists($cacheFile)) {
            return file_get_contents($cacheFile);
        }

        $content = @file_get_contents($url);
        if ($content !== false) {
            file_put_contents($cacheFile, $content);
            return $content;
        }

        return file_exists($cacheFile) ? file_get_contents($cacheFile) : null;
    }

    private function getStory(int $id): ?array
    {
        $cacheFile = self::CACHE_DIR . $id . '.json';
        $content = $this->getFeed(
            self::STORY_BASE_URL . $id . '.json',
            $cacheFile
        );

        return $content ? json_decode($content, true) : null;
    }

    private function generateNanoBananaImage($prompt, $cacheId)
    {
        $image_dir = self::IMAGE_DIR_NANO_BANANA;

        // Clean up old images (older than 30 days)
        $files = glob($image_dir . '*.jpg');
        $yesterday = time() - (24 * 60 * 60 * 30);
        foreach ($files as $file) {
            if (filemtime($file) < $yesterday) {
                unlink($file);
            }
        }

        // Check if image exists
        $image_path = $image_dir . $cacheId . '.jpg';
        if (file_exists($image_path)) {
            return $image_path;
        }

        $url = 'https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent?key=' . self::GEMINI_API_KEY;
        $data = [
            'contents' => [
                [
                    'parts' => [
                        ['text' => $prompt]
                    ]
                ]
            ],
            'generationConfig' => [
                'imageConfig' => [
                    'aspectRatio' => '4:3'
                ]
            ]
        ];

        $headers = [
            'Content-Type: application/json',
            'x-goog-api-key: ' . self::GEMINI_API_KEY
        ];

        $curl = curl_init();
        curl_setopt_array($curl, [
            CURLOPT_URL => $url,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_POST => true,
            CURLOPT_POSTFIELDS => json_encode($data),
            CURLOPT_HTTPHEADER => $headers
        ]);

        $response = curl_exec($curl);
        $err = curl_error($curl);
        curl_close($curl);

        if ($err) {
            return null;
        }

        $result = json_decode($response, true);

        // Extract base64 image data from Gemini response
        $image_data = $result['candidates'][0]['content']['parts'][0]['inlineData']['data'] ?? null;

        if ($image_data) {
            // Decode base64 and save
            $image_content = base64_decode($image_data);
            if ($image_content !== false) {
                file_put_contents($image_path, $image_content);
                return $image_path;
            }
        } else {
            return self::DEFAULT_IMAGE;
        }

        return null;
    }

    private function formatStory(int $id, array $story): array
    {
        $title = htmlspecialchars($story['title']);
        $timestamp = date('M j, Y', $story['time']);
        $score = $story['score'];
        if (isset($story['url'])) {
            $url = htmlspecialchars($story['url']);
        } else {
            $url = "https://news.ycombinator.com/item?id={$id}";
        }
        $image_prompt = sprintf(self::IMAGE_PROMPT_TEMPLATE, $title);
        $image = $this->generateNanoBananaImage($image_prompt, $id);

        $response = array(
            "storyTitle" => $title,
            "storyUrl" => $url,
            "storyImage" => $this->remoteImageBaseUrl . $image,
            "storyTimestamp" => $timestamp,
            "storyId" => $id,
            "storyScore" => $score,
        );

        return $response;
    }

    public function render(): array
    {
        try {
            $content = $this->getFeed(self::BEST_STORIES_URL, $this->bestStoriesCacheFile);
            if (!$content) {
                throw new RuntimeException("Failed to fetch best stories");
            }

            $storyIds = array_slice(json_decode($content, true), 0, self::STORIES_TO_FETCH);

            $stories = [];
            foreach ($storyIds as $id) {
                $story = $this->getStory($id);
                if ($story) {
                    $stories[] = $this->formatStory($id, $story);
                }
            }

            return $stories;

        } catch (Exception $e) {
            error_log($e->getMessage());
            return [];
        }
    }
}

// Usage
$feed = new HackerNewsFeed();
$stories = $feed->render();

$response = [
    'stories' => $stories,
    'metadata' => [
        'totalCount' => count($stories),
        'lastUpdated' => date('c'),
        'version' => '1.0'
    ]
];

$webhookData = [
    'merge_variables' => $response
];

// Check for preview mode
if (isset($_GET['preview']) && $_GET['preview'] === 'true') {
    foreach ($response['stories'] as $story) {
        echo "<img src='" . $story['storyImage'] . "' style='max-width:300px; margin: 10px; display:block;'>";
        echo "<h3>" . $story['storyTitle'] . "</h3>";
        echo "<hr>";
    }
} else {
    // Initialize cURL session
    $ch = curl_init(HackerNewsFeed::WEBHOOK_URL);

    // Set cURL options
    curl_setopt_array($ch, [
        CURLOPT_POST => true,
        CURLOPT_POSTFIELDS => json_encode($webhookData),
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HTTPHEADER => [
            'Content-Type: application/json'
        ]
    ]);

    // Execute request
    $result = curl_exec($ch);

    // Check for errors
    if (curl_errno($ch)) {
        error_log('Webhook Error: ' . curl_error($ch));
    } else {
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        error_log('Webhook Response Code: ' . $httpCode);
        error_log('Webhook Response: ' . $result);
    }

    // Close cURL session
    curl_close($ch);
}

die;
?>