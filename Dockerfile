FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/gostream-broker ./cmd/gostream-broker && \
    go build -o /out/gostream ./cmd/gostream

FROM alpine:3.20
RUN adduser -D -H gostream
WORKDIR /app
COPY --from=build /out/gostream-broker /usr/local/bin/gostream-broker
COPY --from=build /out/gostream /usr/local/bin/gostream
RUN mkdir -p /var/lib/gostream && chown -R gostream:gostream /var/lib/gostream
USER gostream
ENV GOSTREAM_DATA_DIR=/var/lib/gostream
ENV GOSTREAM_HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["gostream-broker"]

