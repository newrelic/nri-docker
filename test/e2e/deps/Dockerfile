FROM alpine:3.15.4

RUN apk add stress-ng

ENTRYPOINT ["/usr/bin/stress-ng"]
CMD ["-c","2", "-l","95", "--hdd","3", "-t","5m"]
