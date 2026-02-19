FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang python3-full
RUN apt-get install -y libsodium-dev
RUN apt install -y python3-pip
RUN python3 -m pip install --break-system-packages -U yt-dlp

RUN apt install -y curl

RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o ~/bin/yt-dlp
RUN chmod a+rx ~/.local/bin/yt-dlp  # Make executable

WORKDIR /app

COPY . /app/

CMD ["./discord_bot"]

