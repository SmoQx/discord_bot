FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang python3-full
RUN apt-get install -y libsodium-dev
RUN apt install -y python3-pip
RUN python3 -m pip install --break-system-packages -U yt-dlp
RUN apt install -y npm
RUN npm install -g deno
WORKDIR /app

COPY . /app/

CMD ["./discord_bot"]

