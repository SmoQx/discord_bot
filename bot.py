import time
import discord
import responses
from discord.ext import commands
import yt_dlp as yt
import asyncio
from urllib.parse import urlparse
import os
from pathlib import Path


semaphore = asyncio.Semaphore(1)


async def send_message(message, user_message, is_private):
    try:
        response = responses.response_message(user_message)
        await message.author.send(response) if is_private else await message.channel.send(response)
    except Exception as e:
        print(e)


def is_valid_url(url):
    try:
        result = urlparse(url)
        return all([result.scheme, result.netloc])
    except ValueError:
        return False


def search_youtube_videos(query):
    ydl_opts = {
        'format': 'best',  # You can adjust the format as needed
        'extract_flat': True,  # Extract only flat results (videos)
        'quiet': True,  # Suppress yt-dlp output
    }

    with yt.YoutubeDL(ydl_opts) as ydl:
        search_results = ydl.extract_info(f'ytsearch:{query}', download=True)

        # Check if 'entries' key exists in the result (videos found)
        if 'entries' in search_results:
            videos = search_results['entries']
            return videos
        else:
            return []


audio_file = './current_audio.mp3'


async def time_and_print_execution_time(func):
    # Record the start time
    start_time = time.time()

    # Run the asynchronous function
    await func

    # Record the end time
    end_time = time.time()

    # Calculate and print the execution time
    elapsed_time = end_time - start_time
    print(f"Execution time: {elapsed_time} seconds")


def run_discord_bot():
    intents = discord.Intents.default()
    intents.message_content = True
    token = 'token_here'
    client = commands.Bot(command_prefix='!', intents=intents)
    play_queue = []
    audio_files_list = []

    ydl_opts = {
        'format': 'bestaudio/best',
        'postprocessors': [{
            'key': 'FFmpegExtractAudio',
            'preferredcodec': 'mp3',
            'preferredquality': '192',
        }],
    }

    async def play_next(ctx):
        global audio_file
        if not play_queue:
            return

        vc = ctx.voice_client
        if vc.is_playing():
            return

        try:
            # with yt.YoutubeDL(ydl_opts) as ydl:
            song = play_queue.pop(0)
            await download_and_save_audio(song)
            voice_client = ctx.voice_client
            print(f"Files in audio file list: \n{audio_files_list}")

            def after_playing(e):
                print(f"PLIK AUDIO TO : {audio_file}")
                if audio_file not in audio_files_list:
                    audio_files_list.append(audio_file)
                if play_queue:
                    asyncio.run(play_next(ctx))
                else:
                    print("play q empty")
                    asyncio.run(cleanup(ctx))

            # Inside the play_next function
            voice_client.stop()
            voice_client.play(discord.FFmpegPCMAudio(audio_file), after=after_playing)

        except Exception as exception_:
            print(exception_)

    async def download_and_save_audio(url):
        global audio_file
        with yt.YoutubeDL(ydl_opts) as ydl:
            info_dict = ydl.extract_info(url, download=False)
            # audio_url = info_dict['formats'][0]['url']
            # Construct a filename using the video ID
            video_id = info_dict['id']
            vide_title = info_dict['fulltitle']
            audio_file = f"{vide_title} [{video_id}].mp3"
            curent_ = Path("current_audio.mp3")

            try:
                # Check if the file exists before renaming
                if os.path.exists(audio_file):
                    return
                else:
                    print(f"File '{audio_file}' not found.")
            except FileNotFoundError as e:
                print(f"Error renaming the file: {e}")

            await asyncio.to_thread(ydl.download, [url])


    @client.command()
    async def cleanup(ctx):
        # Your cleanup code to delete the file.
        files = audio_files_list
        print(files)
        for file in files:
            try:
                if os.path.exists(file):
                    os.remove(file)
                    print(f"Removed {file}")
                    audio_files_list.remove(file)
                elif not os.path.exists(file):
                    pass
                    #print(f"{file} removed from a file list.")
            except FileNotFoundError:
                pass
            except Exception as e:
                print(f"Error while deleting file: {e}")

    @client.command()
    async def list_commands(ctx):
        # Get the list of all registered commands
        command_list = list(client.all_commands.keys())

        # Format the list into a string
        command_list_str = "\n".join(command_list)

        # Send the list of commands as a message
        await ctx.send(f"Available commands:\n```\n{command_list_str}\n```")

    @client.command()
    async def queue(ctx):
        title = ''
        for url in play_queue:
            with yt.YoutubeDL(ydl_opts) as ydl:
                info = ydl.extract_info(url, download=False)
                title = title + '\n' + info['title']
        await ctx.send(f'Songs in q: {title}')

    @client.command()
    async def play(ctx, *url):
        async with semaphore:
            channel = ctx.author.voice.channel

            if ctx.voice_client is None:
                await channel.connect()
            else:
                print("already connected")

            if is_valid_url(url[0]):
                with yt.YoutubeDL(ydl_opts) as ydl:
                    info = ydl.extract_info(url[0], download=False)
                    play_queue.append(url[0])
                    print(f"IS VALID URL{url[0]}")
                    if not ctx.voice_client.is_playing():
                        await play_next(ctx)
                        await ctx.send(f"Now playing: {info['title']}.")
                    else:
                        await ctx.send(f"Added to q: {info['title']}.")
            else:
                await ctx.send("Searching... ")
                query = url
                videos = search_youtube_videos(query)

                if videos:
                    for video in videos:
                        print(f"Video Title: {video['title']}")
                        print(f"Video URL: https://www.youtube.com/watch?v={video['id']}")
                        play_queue.append(f"https://www.youtube.com/watch?v={video['id']}")
                        if not ctx.voice_client.is_playing():

                            await play_next(ctx)
                            await ctx.send(f"Now playing: {video['title']}.")
                        else:
                            await ctx.send(f"Added to q: {video['title']}.")
                else:
                    print("No videos found for the given query.")
                    await ctx.send("No videos found. :c ")

    @client.command()
    async def skip(ctx):
        # Check if there are songs in the queue
        global audio_file
        voice_client = ctx.voice_client
        if not play_queue:
            await ctx.send("The play queue is empty.")
            voice_client.stop()
            return
        voice_client.stop()
        # await play_next(ctx)
        # Get the next song URL from the queue
        """next_song = play_queue.pop(0)

        with yt.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(next_song, download=False)
            url2 = info['url']
            voice_client.stop()
            voice_client.play(discord.FFmpegPCMAudio(url2))

        await ctx.send(f"Now playing: {info['title']}")"""

    @client.command()
    async def stop(ctx):
        voice_client = ctx.voice_client
        voice_client.stop()

    @client.event
    async def on_ready():
        print(f'{client.user} is now running!')

    @client.command()
    async def join(ctx):
        # Check if the user is in a voice channel
        if ctx.author.voice is None:
            await ctx.send("You are not in a voice channel.")
            return

        # Connect to the user's voice channel
        channel = ctx.author.voice.channel
        await channel.connect()
        await ctx.send(f"Joined {channel.name}")

    @client.command()
    async def leave(ctx):
        voice_client = ctx.guild.voice_client

        if voice_client:
            await voice_client.disconnect()
            await ctx.send("Left the voice channel.")
        else:
            await ctx.send("I'm not in a voice channel.")

    client.run(token)
    asyncio.run(cleanup(0))