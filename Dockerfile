FROM golang:1.20 as builder

# Setting up sre-tools build
COPY . $GOPATH/src/syossan27/pre-oom-killer/
WORKDIR $GOPATH/src/syossan27/pre-oom-killer/
RUN go mod download

# Running sre-tools build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/pre-oom-killer

# Starting on Scratch
FROM scratch

# Moving needed binaries to
COPY --from=builder /go/bin/pre-oom-killer /go/bin/pre-oom-killer

ENTRYPOINT ["/go/bin/pre-oom-killer"]
