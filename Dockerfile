FROM --platform=$BUILDPLATFORM golang:1.15

ARG BUILDPLATFORM
ARG TARGETARCH
ARG TARGETOS

WORKDIR /go/src/github.com/wish/node2pod

# Cache dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /go/src/github.com/wish/node2pod
# Build controller
RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} GOOS=${TARGETOS} go build -o /bin/node2pod ./main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=0 /bin/node2pod /bin/node2pod
