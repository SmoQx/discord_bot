FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang

WORKDIR /app

COPY . /app/

CMD ["go", "run", "bot.go"]

