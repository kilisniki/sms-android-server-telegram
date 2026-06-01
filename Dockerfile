FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/sms-server ./cmd/server/main.go

FROM alpine:latest
RUN apk add --no-cache android-tools tzdata

WORKDIR /app
COPY --from=builder /app/sms-server .

# Create volume for db
VOLUME ["/app/data"]

CMD ["./sms-server"]
