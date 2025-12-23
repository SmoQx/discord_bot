FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang
RUN apt-get install -y libsodium-dev

WORKDIR /app

COPY . /app/

CMD ["./discord_bot"]

