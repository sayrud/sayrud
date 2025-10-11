FROM docker.cnb.cool/nekobase/base-images/alpine-runtime:latest

COPY ./sayrud-server .

RUN chmod 777 /home/app/sayrud-server

ENTRYPOINT ["./sayrud-server"]
EXPOSE 8080
