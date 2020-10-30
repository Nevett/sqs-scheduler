FROM golang:alpine AS build

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY . .
RUN go mod download
RUN go build -o consumer cmd/consumer/main.go
RUN go build -o producer cmd/producer/main.go

FROM scratch AS producer
COPY --from=korbin/dockerize /usr/local/bin/dockerize /
COPY --from=build build/producer /
ENTRYPOINT ["/dockerize", "-wait", "file:///ready/localstack", "-timeout", "60s", "/producer"]

FROM scratch AS consumer
COPY --from=korbin/dockerize /usr/local/bin/dockerize /
COPY --from=build build/consumer /
ENTRYPOINT ["/dockerize", "-wait", "file:///ready/localstack", "-timeout", "60s", "/consumer"]