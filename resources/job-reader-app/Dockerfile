FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o job-reader main.go

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/job-reader /job-reader
RUN mkdir -p /data
EXPOSE 8080
ENTRYPOINT ["/job-reader"]