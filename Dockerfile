FROM docker.cnb.cool/nekobase/base-images/alpine-runtime:latest

RUN apk update && apk add tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
&& echo "Asia/Shanghai" > /etc/timezone

WORKDIR /home/app

COPY ./sayrud-server .

RUN chmod 777 /home/app/sayrud-server

ENTRYPOINT ["./sayrud-server"]
EXPOSE 8080
