ARG PKGNAME

# Build the manager binary
FROM golang:1.17.2-alpine as builder
ENV GOPROXY https://proxy.golang.org,direct
ENV GOSUMDB sum.golang.org

ARG LDFLAGS
ARG PKGNAME

WORKDIR /go/src/github.com/gocrane/fadvisor
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY pkg pkg/
COPY cmd cmd/
COPY staging staging/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -a -o ${PKGNAME} /go/src/github.com/gocrane/fadvisor/cmd/${PKGNAME}/main.go

FROM alpine:3.13.5
WORKDIR /
ARG PKGNAME
COPY --from=builder /go/src/github.com/gocrane/fadvisor/${PKGNAME} .