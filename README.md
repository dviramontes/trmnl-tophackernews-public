# TRMNL Top Hacker News

Top Hacker News stories for your [TRMNL](https://usetrmnl.com/) e-ink display.

![img_UcO3HHHE](https://github.com/user-attachments/assets/f9aa15a3-7387-4435-b8be-45900bd40127)

This PHP script fetches the top 5 stories from Hacker News, generates unique "gritty noir comic book" style illustrations for each headline using Google's Gemini (Nano Banana) API, and pushes the content to a TRMNL Custom Plugin.

## Features

- **Automated Content**: Fetches real-time "Best Stories" from the Hacker News API.
- **AI Art Generation**: Uses Google Gemini 2.5 Flash to generate unique artwork for every story.
- **Caching**: Caches stories and generated images to minimize API usage and improve performance.
- **Preview Mode**: View the generated content in your browser without pushing to TRMNL.
- **TRMNL Integration**: Formats and pushes data directly to your TRMNL device via webhook.

## Requirements

- PHP 7.4 or higher
- `curl` extension enabled
- Write permissions for the script directory (for caching and saving images)
- A Google Gemini API Key
- A TRMNL Custom Plugin Webhook URL

## Setup

1.  **Clone the repository:**

2.  **Configure variabless:**
    Open `index.php` and update all following variables at the top of the class, for example:

    ```php
    private const GEMINI_API_KEY = 'YOUR_GEMINI_API_KEY';
    public const WEBHOOK_URL = 'YOUR_TRMNL_WEBHOOK_URL';
    ```

3.  **Permissions:**
    Ensure the script has permission to create directories and write files:
    ```bash
    chmod 755 .
    ```
    The script will automatically create `cache/` and `headline_images_nano_banana/` directories.

## Usage

### Automating with Cron

To keep your TRMNL display updated, set up a cron job to run the script periodically (e.g., every 4 hours):

```bash
0 */4 * * * /usr/bin/php /path/to/trmnl-tophackernews/index.php >/dev/null 2>&1
```

### Preview Mode

To visualize the output in your browser without sending data to TRMNL, use the `preview` parameter:

`https://your-domain.com/index.php?preview=true`

### Force Update

To bypass the cache and force a fresh fetch of stories and images:

`https://your-domain.com/index.php?update=true`

## Go Version

A Go implementation is also available in this repository. See `main.go` and the `Justfile` for usage:

```bash
just run

# Just by itself for other commands
just 
```

Set `GEMINI_API_KEY` environment variable before running.
