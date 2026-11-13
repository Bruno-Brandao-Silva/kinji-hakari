# gopus compiles bundled libopus via cgo, so the build stage needs a C toolchain.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/hakari-bot .

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --no-create-home hakari
WORKDIR /app
COPY --from=build /out/hakari-bot ./hakari-bot
COPY tuca-donka.mp3 ./tuca-donka.mp3
USER hakari
CMD ["./hakari-bot"]
