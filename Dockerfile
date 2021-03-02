FROM golang:1.16-alpine AS build
COPY ./go.mod ./go.sum ./
ENV GOPATH ""
RUN go mod download
COPY . .
RUN go build -o /secrets-init -v

FROM alpine:3.10

COPY --from=build /secrets-init /usr/local/bin/secrets-init
RUN chmod +x /usr/local/bin/secrets-init 

CMD ["secrets-init", "--version"]