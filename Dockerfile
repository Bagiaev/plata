FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 go build -trimpath -o /out/plata-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/plata-migrate ./cmd/migrate

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -H app
COPY --from=build /out/plata-server /out/plata-migrate /usr/local/bin/
USER app
EXPOSE 8080
CMD ["/usr/local/bin/plata-server"]
