FROM golang:1.25-alpine AS builder
WORKDIR /src
# Cache dependencies separately from code.
COPY go.mod go.sum ./
RUN go mod download
# Build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /headtotails ./cmd/headtotails

# ---- Final image ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /headtotails /usr/local/bin/headtotails

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/headtotails"]
