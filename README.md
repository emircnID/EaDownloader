# EaDownloader

EaDownloader is a lightweight Telegram bot for downloading public media from
popular social platforms. It is designed to run as a public bot with Docker
Compose, PostgreSQL, cached media delivery, and optional cookie-based
authentication for providers that require it.

## Supported Platforms

- Instagram
- TikTok
- X / Twitter
- Facebook
- YouTube

YouTube links support interactive format selection. Users can choose video
quality such as 360p, 720p, 1080p, or request an MP3 audio download.

## Features

- Public bot mode by default
- English and Turkish localization
- Telegram media caching for faster repeat delivery
- Optional captions and source links
- Optional whitelist and admin diagnostics
- Docker Compose deployment with PostgreSQL
- Optional Netscape cookie files for YouTube, Facebook, and X
- Local-only metrics and profiler ports

## Requirements

- A Linux server
- Docker and Docker Compose
- A Telegram bot token from BotFather
- A PostgreSQL database, provided by the included Docker Compose stack

## Quick Start

Install Docker and Docker Compose:

```bash
sudo apt update
sudo apt install -y git docker.io docker-compose-plugin
```

Clone the repository:

```bash
git clone https://github.com/emircnID/EaDownloader.git
cd EaDownloader
```

Create your environment file:

```bash
cp .env.example .env
nano .env
```

At minimum, update these values:

```env
BOT_TOKEN=your-telegram-bot-token
DB_PASSWORD=use-a-strong-password
ADMINS=your-telegram-user-id
```

Start the bot:

```bash
docker compose pull
docker compose up -d
docker compose logs -f bot
```

If your user is not in the Docker group, run the Docker commands with `sudo`.

## Updating

Pull the latest code and container image, then recreate the services:

```bash
git pull
docker compose pull
docker compose up -d --force-recreate
```

## Cookie Files

Most public links are attempted without cookies first. Some content may still
require authentication, especially when a provider blocks datacenter IP
addresses, asks for age verification, or requires a signed-in session.

Place Netscape-format cookie files in the following paths:

```text
private/cookies/youtube.txt
private/cookies/facebook.txt
private/cookies/twitter.txt
```

The `private` directory is mounted into the container and ignored by Git, so
real cookie files and private configuration should stay on the server only.

After changing cookie files, recreate the bot container:

```bash
docker compose up -d --force-recreate bot
```

## Configuration Notes

- `MAX_FILE_SIZE` is configured in megabytes.
- `WHITELIST` can be left empty for public bot mode.
- `METRICS_PORT` and `PROFILER_PORT` are bound to `127.0.0.1` by default.
- `CACHING=true` allows Telegram file IDs to be reused for faster repeat sends.
- `DEFAULT_LANGUAGE=tr` starts chats in Turkish by default.

## Development

Development helpers are included for local work:

- `Dockerfile.dev`
- `docker-compose.dev.yaml`
- `.air.toml`
- `sqlc.yaml`
- `.golangci.yml`

Run the development stack with:

```bash
docker compose -f docker-compose.dev.yaml up --build
```

## License

This project is distributed under the terms of the repository license.
