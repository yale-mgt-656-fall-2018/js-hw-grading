# This dockerfile is taken from 
# https://blog.hasura.io/the-ultimate-guide-to-writing-dockerfiles-for-go-web-apps-336efad7012c
FROM golang:1.10-alpine as builder
WORKDIR /go/src/github.com/yale-mgt-656-fall-2018/js-hw-grading
# add source code
COPY main.go main.go
COPY grading grading
COPY vendor vendor

RUN apk --no-cache add ca-certificates
# RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

# build the source
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main && chmod a+x main

# use scratch (base for a docker image)
FROM scratch
# set working directory
WORKDIR /root
# copy the binary from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/github.com/yale-mgt-656-fall-2018/js-hw-grading/main /root
ENTRYPOINT ["/root/main"]