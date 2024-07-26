FROM golang:1.22-alpine as go_builder

WORKDIR /app

ENV CGO_ENABLED=0

COPY . .

RUN go mod tidy
RUN go build -v -trimpath -ldflags "-w -s -extldflags '-static' -X 'github.com/wuhan005/sayrud/internal/appconst.BuildCommit=$GITHUB_SHA'" -o sayrud-server ./cmd/sayrud-server

FROM alpine:latest

RUN apk update && apk add tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
&& echo "Asia/Shanghai" > /etc/timezone

WORKDIR /home/app

COPY --from=go_builder /app/sayrud-server .

RUN chmod 777 /home/app/sayrud-server

ENTRYPOINT ["./sayrud-server"]
EXPOSE 8080
