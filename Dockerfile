FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o /out/nordikcsaaapi ./cmd/server

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=build /out/nordikcsaaapi /app/nordikcsaaapi

USER app
EXPOSE 8080

ENTRYPOINT ["/app/nordikcsaaapi"]
