FROM golang:1.26-alpine AS build
WORKDIR /src
# tzdata: parsePublishedOn needs Asia/Kathmandu
RUN apk add --no-cache tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/backfill ./cmd/backfill

FROM scratch
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/api /api
COPY --from=build /out/backfill /backfill
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/api"]
