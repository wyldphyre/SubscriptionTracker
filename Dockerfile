FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata && mkdir -p /data
COPY subtracker /subtracker
ENTRYPOINT ["/subtracker"]
