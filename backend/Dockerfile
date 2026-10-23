# Build context is the backend/ folder alone, so this service builds and ships
# without the rest of the repository:  docker build -t nexa ./backend
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
# cgo is required: H3 (github.com/uber/h3-go) wraps the C library. The runtime image
# below is the distroless glibc base of the same Debian release as this builder.
RUN CGO_ENABLED=1 go build -ldflags "-s -w -X main.Version=${VERSION}" -o /out/nexa ./cmd/nexa
# Healthcheck helper: the runtime image has no shell or curl (see tools/probe).
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/probe ./tools/probe

FROM gcr.io/distroless/base-debian13
WORKDIR /app
COPY --from=build /out/nexa /app/nexa
COPY --from=build /out/probe /app/probe
COPY --from=build /src/brands /app/brands
COPY --from=build /src/seed /app/seed
COPY --from=build /src/i18n /app/i18n
ENV NEXA_BRANDS_DIR=/app/brands NEXA_SEED_DIR=/app/seed NEXA_I18N_DIR=/app/i18n NEXA_HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=4s --retries=3 CMD ["/app/probe"]
ENTRYPOINT ["/app/nexa"]
CMD ["serve"]
