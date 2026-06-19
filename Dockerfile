FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o /out/nordikcsaaapi ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache fontconfig ttf-dejavu && addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=build /out/nordikcsaaapi /app/nordikcsaaapi
COPY --from=build /src/internal/books/fonts /app/fonts

ENV BOOK_FONT_DIR=/app/fonts

USER app
EXPOSE 8080

ENTRYPOINT ["/app/nordikcsaaapi"]
