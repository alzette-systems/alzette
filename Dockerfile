# syntax=docker/dockerfile:1
FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/alzette ./cmd/alzette

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/alzette /usr/local/bin/alzette
COPY login.html login.css portal.html portal.css portal.js alzette-mark.svg /app/portal/
COPY index.html docs.html site.css alzette-mark.svg /app/public/

USER nonroot:nonroot
EXPOSE 8080 8081 8082 8083
ENTRYPOINT ["/usr/local/bin/alzette"]
CMD ["serve", "--addr", ":8080", "--static-dir", "/app/portal"]
