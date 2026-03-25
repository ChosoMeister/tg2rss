# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod/ \
  go mod download -x

ARG TARGETARCH

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod/ \
  CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/tg2rss cmd/tg2rss/main.go

FROM alpine:latest AS final

RUN --mount=type=cache,target=/var/cache/apk \
  apk --update add \
  ca-certificates \
  tzdata \
  && \
  update-ca-certificates

COPY --from=build /bin/tg2rss /app/
WORKDIR /app
CMD ["./tg2rss"]
