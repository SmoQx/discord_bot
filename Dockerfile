FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang python3
RUN apt-get install -y libsodium-dev
RUN python3 -m pip install --no-deps -U yt-dlp

WORKDIR /app

COPY . /app/

CMD ["./discord_bot"]

