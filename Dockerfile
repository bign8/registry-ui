FROM golang:1.10-alpine as go
WORKDIR /go/src/
RUN apk add --update upx
ADD /main.go ./
RUN CGO_ENABLED=0 go build -o registry-ui -ldflags="-s -w" -v
RUN upx --ultra-brute registry-ui

FROM scratch
EXPOSE 8080
COPY --from=go /go/src/registry-ui /
ENTRYPOINT ["/registry-ui"]
