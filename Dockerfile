# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

ARG GOPROXY=https://goproxy.io,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /nest-cash ./cmd/nest-cash

# Run stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /nest-cash /usr/local/bin/nest-cash

EXPOSE 8080
CMD ["nest-cash"]
