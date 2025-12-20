`tgfeed` converts Telegram channels into RSS or Atom feeds suitable for any RSS reader of your choice. It runs as an HTTP server that dynamically scrapes t.me channel pages and generates RSS or Atom feeds on demand.

## Usage

Running using Docker:

```shell
$ docker compose up -d
```

This will start the tgfeed server on port 8080 (can be changed via HTTP_SERVER_PORT environment variable).

## API Endpoints

### Get Channel Feed

```
GET /telegram/channel/{username}
```

Generates a feed for the specified Telegram channel.

#### Path Parameters

- `username` - Telegram channel username (required)

#### Query Parameters

- `format` - Feed format, either "rss" or "atom" (default: "rss")
- `exclude` - List of words to exclude posts containing them, separated by `|` (optional)
- `exclude_case_sensitive` - Whether to match excluded words case-sensitively, "1" or "true" for case-sensitive (default: false)
- `cache_ttl` - Cache TTL in minutes, 0 to disable caching (default: 60)

#### Example

```
# Get RSS feed for "durov" channel
http://localhost:8080/telegram/channel/durov

# Get Atom feed for "durov" channel with exclusions
http://localhost:8080/telegram/channel/durov?format=atom&exclude=crypto|bitcoin

# Get RSS feed with no caching
http://localhost:8080/telegram/channel/durov?cache_ttl=0
```

## Environment Variables

- `UNSUPPORTED_MESSAGE_HTML` - Custom HTML message for unsupported post content. Use `{postDeepLink}` and `{postURL}` as placeholders for post links.
- `IMAGE_POST_TITLE_TEXT` - Custom title for image-only posts (default: "[🖼️ Image]")
- `USER_AGENT` - Custom User-Agent header for requests to t.me
- `HTTPS_PROXY` - HTTP proxy for accessing t.me if needed
- `ALLOWED_IPS` - Comma-separated list of allowed IP addresses or CIDR ranges (e.g., "10.0.0.0/24,192.168.1.1"). If not set, all IPs are allowed.
- `REVERSE_PROXY` - Set to "true" or "1" to trust X-Real-IP and X-Forwarded-For headers for IP extraction. Only enable this if ALLOWED_IPS are configured and tgfeed is behind a reverse proxy. (default: false)

## Example RSS Reader Configuration

When adding a feed to your RSS reader, use the URL:

```
http://your-server:8080/telegram/channel/channelname
```

Replace `channelname` with the username of the Telegram channel you want to follow.

## Docker Compose

The service can be run using Docker Compose. Customize the configuration through environment variables in the `compose.yaml` file. You can uncomment some config values there if you want to keep cache in Redis. Otherwise it will be kept in RAM (by default).