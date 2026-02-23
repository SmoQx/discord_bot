FROM ubuntu:latest

RUN apt update && apt install -y ffmpeg bash golang python3-full
RUN apt-get install -y libsodium-dev
RUN apt install -y python3-pip

WORKDIR /app

COPY . /app/

#RUN mv ./yt-dlp_linux /bin/yt-dlp
#RUN chmod a+rx /bin/yt-dlp  # Make executable

RUN python3 ./yt-dlp-master/devscripts/install_deps.py --include-extra pyinstaller
RUN python3 ./yt-dlp-master/devscripts/make_lazy_extractors.py
RUN python3 -m bundle.pyinstaller


CMD ["./discord_bot"]

