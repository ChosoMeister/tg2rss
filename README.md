# tg2rss

> Convert any public Telegram channel into an RSS or Atom feed.

`tg2rss` is a lightweight, self-hosted HTTP server that converts public Telegram channels into standard RSS/Atom feeds. No Telegram Bot API key required — it works by scraping public channel pages (`t.me/s/channel`).

**Key features:** Redis caching · gzip compression · ETag/304 support · stale-while-revalidate · graceful degradation · Prometheus metrics · rate limiting · IP filtering · nginx-ready

---

## Table of Contents

- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [API Reference](#api-reference)
  - [Get Channel Feed](#get-channel-feed)
  - [Health Check](#health-check)
  - [Prometheus Metrics](#prometheus-metrics)
- [Configuration](#configuration)
- [Performance Features](#performance-features)
- [Running Behind nginx](#running-behind-nginx)
- [Docker Compose](#docker-compose)
- [فارسی (Persian)](#-راهنمای-فارسی)

---

## Quick Start

```shell
docker compose up -d
```

That's it! tg2rss is now running on port **8080** with Redis caching. Add this URL to your RSS reader:

```
http://localhost:8080/telegram/channel/durov
```

Replace `durov` with any public Telegram channel username.

---

## How It Works

```
┌─────────────┐      ┌─────────┐      ┌──────────────┐
│  RSS Reader  │ ───→ │  tg2rss │ ───→ │  Telegram    │
│  (Feedly,    │ ←─── │         │ ←─── │  (t.me)      │
│   Miniflux)  │      │         │      └──────────────┘
└─────────────┘      │         │
                      │    ↕    │
                      │  Redis  │
                      │ (cache) │
                      └─────────┘
```

1. Your RSS reader requests a feed URL (e.g., `/telegram/channel/durov`)
2. tg2rss checks Redis cache — if the feed was generated recently, it returns it **instantly**
3. If the cache expired, tg2rss **serves the old feed immediately** while refreshing in the background (stale-while-revalidate)
4. If there's no cache at all, tg2rss scrapes the Telegram channel page and generates a fresh RSS/Atom feed
5. If Telegram is unreachable (down, blocked, etc.), tg2rss returns the **last known cached version** instead of an error (graceful degradation)

---

## API Reference

### Get Channel Feed

```http
GET /telegram/channel/{username}
```

Generates an RSS or Atom feed for the specified public Telegram channel.

#### Parameters

| Parameter | Type | Description | Default |
|---|---|---|---|
| `username` | path | Telegram channel username | **required** |
| `format` | query | Feed format: `rss` or `atom` | `rss` |
| `exclude` | query | Filter out posts containing these words (separated by `\|`) | — |
| `exclude_case_sensitive` | query | Case-sensitive word matching (`true` or `1`) | `false` |
| `cache_ttl` | query | Cache lifetime in minutes (`0` = no cache) | `60` |

#### Response Headers

| Header | Description |
|---|---|
| `Content-Type` | `application/rss+xml` or `application/atom+xml` |
| `X-CACHE-STATUS` | `HIT` (from cache), `MISS` (freshly generated), or `STALE` (served from expired cache) |
| `X-Data-Stale` | `true` if the response is from expired cache (fallback) |
| `ETag` | Content hash for conditional requests |
| `Cache-Control` | `public, max-age=N` where N = cache_ttl × 60 |

#### Examples

```bash
# Basic RSS feed for a channel
curl http://localhost:8080/telegram/channel/durov

# Atom format instead of RSS
curl http://localhost:8080/telegram/channel/durov?format=atom

# Filter out posts containing "crypto" or "bitcoin" (case-insensitive)
curl "http://localhost:8080/telegram/channel/durov?exclude=crypto|bitcoin"

# Same but case-sensitive
curl "http://localhost:8080/telegram/channel/durov?exclude=Crypto|Bitcoin&exclude_case_sensitive=true"

# Disable caching — always fetch fresh from Telegram
curl http://localhost:8080/telegram/channel/durov?cache_ttl=0

# Cache for 2 hours instead of the default 60 minutes
curl http://localhost:8080/telegram/channel/durov?cache_ttl=120

# Use with conditional request (returns 304 if unchanged)
curl -H "If-None-Match: \"abc123\"" http://localhost:8080/telegram/channel/durov
```

---

### Health Check

```http
GET /health
```

Returns a simple JSON health status. Use this for load balancer health checks, Docker health checks, or uptime monitoring.

```json
{"status": "ok"}
```

---

### Prometheus Metrics

```http
GET /metrics
```

Only available when `METRICS_ENABLED=true`. Exposes Prometheus-compatible metrics:

| Metric | Type | Description |
|---|---|---|
| `tg2rss_http_requests_total` | Counter | Total HTTP requests (labels: method, path, status) |
| `tg2rss_http_request_duration_seconds` | Histogram | Request processing time |
| `tg2rss_cache_hits_total` | Counter | Number of cache hits |
| `tg2rss_cache_misses_total` | Counter | Number of cache misses |
| `tg2rss_scrape_duration_seconds` | Histogram | Time spent scraping Telegram |
| `tg2rss_active_scrapes` | Gauge | Currently active scrape operations |
| `tg2rss_stale_responses_total` | Counter | Stale/fallback responses served |

---

## Configuration

All configuration is done through environment variables. The `compose.yaml` file contains detailed inline documentation for each option.

| Variable | Description | Default |
|---|---|---|
| **Server** | | |
| `HTTP_SERVER_PORT` | Port the HTTP server listens on | `8080` |
| `BASE_PATH` | URL prefix for all routes (e.g., `/feeds`) | — |
| **Caching** | | |
| `REDIS_HOST` | Redis hostname. If not set, uses in-memory cache (lost on restart) | — |
| `DEFAULT_CACHE_TTL` | Default cache lifetime in minutes | `60` |
| **Reverse Proxy** | | |
| `REVERSE_PROXY` | Set to `true` when behind nginx/Caddy/Traefik to read real client IPs | `false` |
| **Security** | | |
| `ALLOWED_IPS` | Comma-separated allowed IPs or CIDR ranges (e.g., `10.0.0.0/24,1.2.3.4`) | all allowed |
| `RATE_LIMIT` | Max requests per second per IP address. `0` = disabled | `0` |
| `RATE_BURST` | Rate limit burst size (how many quick requests are tolerated) | auto |
| **Monitoring** | | |
| `METRICS_ENABLED` | Enable Prometheus `/metrics` endpoint (`true` or `1`) | `false` |
| **Performance** | | |
| `MAX_CONCURRENT_SCRAPES` | Max simultaneous requests to Telegram | `3` |
| **Connectivity** | | |
| `HTTPS_PROXY` | HTTP/SOCKS proxy for reaching Telegram (e.g., `socks5://proxy:1080`) | — |
| `USER_AGENT` | Custom User-Agent string for Telegram requests | Chrome UA |
| **Content** | | |
| `UNSUPPORTED_MESSAGE_HTML` | HTML for posts that can't be displayed. Placeholders: `{postDeepLink}`, `{postURL}` | built-in |
| `IMAGE_POST_TITLE_TEXT` | Title for image-only posts (no text content) | `[🖼️ Image]` |

---

## Performance Features

| Feature | How it works |
|---|---|
| **Gzip Compression** | Responses larger than 1KB are automatically gzip-compressed when the client sends `Accept-Encoding: gzip` |
| **ETag / 304 Not Modified** | Each response includes an `ETag` header. When the client sends `If-None-Match`, the server returns `304` if content is unchanged — saving bandwidth |
| **Stale-While-Revalidate** | When cache expires, the old content is served instantly while a background goroutine fetches fresh data from Telegram |
| **Graceful Degradation** | If Telegram is down or blocked, the last known cached feed is returned with `X-Data-Stale: true` instead of an error |
| **Parallel Image Loading** | Image metadata (sizes) are fetched concurrently using goroutines instead of one-by-one |
| **Redis Caching** | Feed XML is stored in Redis with configurable TTL. Stale entries are kept for 24 extra hours as fallback |

---

## Running Behind nginx

tg2rss is designed to work behind a reverse proxy. Make sure to set `REVERSE_PROXY=true` so the app reads real client IPs from proxy headers.

### Basic Setup

```nginx
upstream tg2rss {
    server 127.0.0.1:8080;
}

server {
    listen 443 ssl http2;
    server_name feeds.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://tg2rss;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health check — no access logs
    location /health {
        proxy_pass http://tg2rss;
        access_log off;
    }
}
```

### Serving on a Subpath

To serve tg2rss at `https://example.com/feeds/...`, set `BASE_PATH=/feeds`:

```nginx
location /feeds/ {
    proxy_pass http://tg2rss;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

Feed URLs become: `https://example.com/feeds/telegram/channel/durov`

---

## Docker Compose

The included `compose.yaml` runs tg2rss with Redis out of the box. Every option is documented with detailed comments inside the file.

```shell
# Start in background
docker compose up -d

# View real-time logs
docker compose logs -f tg2rss

# Restart after config changes
docker compose up -d --force-recreate

# Stop everything
docker compose down

# Stop and delete cached data
docker compose down -v
```

---

---

# 🇮🇷 راهنمای فارسی

## tg2rss چیه؟

یه ابزار سبک و self-hosted که کانال‌های عمومی تلگرام رو به فید RSS/Atom تبدیل می‌کنه. نیازی به API Key تلگرام نداره — از صفحات عمومی (`t.me/s/channel`) اسکرپ می‌کنه.

## شروع سریع

```shell
docker compose up -d
```

همین! حالا تو RSS Reader خودت (مثل Feedly, Miniflux, FreshRSS) این آدرس رو اضافه کن:

```
http://localhost:8080/telegram/channel/durov
```

به جای `durov` اسم هر کانال عمومی تلگرام رو بذار.

## نحوه کار

```
┌───────────────┐      ┌─────────┐      ┌──────────────┐
│  RSS Reader   │ ───→ │  tg2rss │ ───→ │   تلگرام      │
│  (Feedly,     │ ←─── │         │ ←─── │  (t.me)      │
│   Miniflux)   │      │         │      └──────────────┘
└───────────────┘      │    ↕    │
                        │  Redis  │
                        │ (کش)    │
                        └─────────┘
```

1. RSS Reader شما یه فید درخواست می‌کنه
2. اگه فید قبلاً ساخته شده و توی کش هست → **فوری** برمی‌گردونه
3. اگه کش منقضی شده → فید قدیمی رو **فوری** می‌ده و پشت صحنه آپدیت می‌کنه
4. اگه اصلاً کشی نیست → از تلگرام اسکرپ می‌کنه و فید تازه می‌سازه
5. اگه تلگرام در دسترس نباشه → آخرین نسخه کش‌شده رو برمی‌گردونه (به جای خطا)

## آدرس‌های API

### دریافت فید کانال

```
GET /telegram/channel/{username}
```

| پارامتر | توضیح | مقدار پیش‌فرض |
|---|---|---|
| `username` | نام کاربری کانال تلگرام (در URL) | **الزامی** |
| `format` | فرمت فید: `rss` یا `atom` | `rss` |
| `exclude` | کلماتی که پست‌های حاوی آن‌ها حذف بشن (جدا با `\|`) | — |
| `cache_ttl` | مدت کش به دقیقه (`0` = بدون کش) | `60` |

**مثال‌ها:**

```bash
# فید RSS ساده
curl http://localhost:8080/telegram/channel/durov

# فرمت Atom
curl http://localhost:8080/telegram/channel/durov?format=atom

# فیلتر کردن پست‌ها
curl "http://localhost:8080/telegram/channel/durov?exclude=crypto|bitcoin"

# بدون کش (همیشه از تلگرام بخونه)
curl http://localhost:8080/telegram/channel/durov?cache_ttl=0
```

### بررسی سلامت

```
GET /health
```

جواب: `{"status":"ok"}` — برای health check داکر و مانیتورینگ.

## تنظیمات

تمام تنظیمات از طریق متغیرهای محیطی (environment variables) انجام میشه. فایل `compose.yaml` شامل توضیحات کامل هر گزینه با مثال هست.

| متغیر | توضیح | پیش‌فرض |
|---|---|---|
| `HTTP_SERVER_PORT` | پورت سرور | `8080` |
| `REDIS_HOST` | آدرس Redis (بدونش از حافظه RAM استفاده میشه) | — |
| `DEFAULT_CACHE_TTL` | مدت کش به دقیقه | `60` |
| `BASE_PATH` | پیشوند آدرس (مثلا `/feeds`) | — |
| `REVERSE_PROXY` | پشت nginx هستی؟ `true` بذار | `false` |
| `ALLOWED_IPS` | فقط این IPها اجازه دسترسی دارن | همه |
| `RATE_LIMIT` | حداکثر درخواست در ثانیه برای هر IP | `0` (غیرفعال) |
| `METRICS_ENABLED` | فعال‌سازی Prometheus metrics | `false` |
| `MAX_CONCURRENT_SCRAPES` | حداکثر درخواست همزمان به تلگرام | `3` |
| `HTTPS_PROXY` | پروکسی برای دسترسی به تلگرام | — |

## قابلیت‌های عملکردی

| قابلیت | توضیح |
|---|---|
| **فشرده‌سازی Gzip** | پاسخ‌های بزرگتر از 1KB خودکار فشرده میشن |
| **ETag / 304** | اگه فید تغییر نکرده باشه، `304 Not Modified` برمی‌گردونه (صرفه‌جویی در پهنای باند) |
| **Stale-While-Revalidate** | فید قدیمی فوری سرو میشه و پشت صحنه آپدیت میشه |
| **Graceful Degradation** | اگه تلگرام دان باشه، آخرین فید کش‌شده رو میده (به جای خطا) |
| **بارگذاری موازی تصاویر** | اطلاعات تصاویر همزمان (نه یکی‌یکی) دریافت میشن |

## نصب پشت nginx

اگه از nginx استفاده می‌کنی، حتماً `REVERSE_PROXY=true` رو ست کن:

```nginx
server {
    listen 443 ssl;
    server_name feeds.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## دستورات Docker

```shell
# اجرا در پس‌زمینه
docker compose up -d

# مشاهده لاگ‌ها
docker compose logs -f tg2rss

# توقف
docker compose down
```