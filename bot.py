import discord
import responses
from discord.ext import commands
import yt_dlp as yt
import asyncio
import spotipy
from spotipy.oauth2 import SpotifyOAuth
from urllib.parse import urlparse


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
        search_results = ydl.extract_info(f'ytsearch:{query}', download=False)

        # Check if 'entries' key exists in the result (videos found)
        if 'entries' in search_results:
            videos = search_results['entries']
            return videos
        else:
            return []


def run_discord_bot():
    intents = discord.Intents.default()
    intents.message_content = True
    token = 'NjkwOTkyNjM4NjM5MzQxNjI5.GXNezh.MBI7BhO6auPjtbwYq6orlA07voB73Ar4MMGdv8'
    client = commands.Bot(command_prefix='!', intents=intents)
    play_queue = []

    ydl_opts = {
        'format': 'bestaudio/best',
        'postprocessors': [{
            'key': 'FFmpegExtractAudio',
            'preferredcodec': 'mp3',
            'preferredquality': '192',
        }],
    }

    # Initialize the Spotify API client
    sp = spotipy.Spotify(auth_manager=SpotifyOAuth(client_id='fbe4ddd2b0944d8caadd82fc3a634ba6',
                                                   client_secret='937f97778b604fa8a3367d81979d23c8',
                                                   redirect_uri='http://127.0.0.1:8888/callback',
                                                   scope='user-library-read user-read-playback-state user-modify-playback-state'))

    async def play_next(ctx):
        if not play_queue:
            return

        vc = ctx.voice_client
        if vc.is_playing():
            return

        try:
            with yt.YoutubeDL(ydl_opts) as ydl:
                song = play_queue.pop(0)
                info = ydl.extract_info(song, download=False)
                url2 = info['url']
                voice_client = ctx.voice_client

                def after_playing(e):
                    # This function is called when the song has finished playing
                    asyncio.run_coroutine_threadsafe(play_next(ctx), client.loop)

                voice_client.stop()
                voice_client.play(discord.FFmpegPCMAudio(url2), after=after_playing)
        except Exception as exception_:
            print(exception_)
        await ctx.send(f"Now playing: {info['title']}.")

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
                play_queue.append(url[0])
                print(f"IS VALID URL{url[0]}")
                if not ctx.voice_client.is_playing():
                    await play_next(ctx)
                else:
                    with yt.YoutubeDL(ydl_opts) as ydl:
                        info = ydl.extract_info(url[0], download=False)
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
                        await play_next(ctx)
                else:
                    print("No videos found for the given query.")
                    await ctx.send("No videos found. :c ")

    @client.command()
    async def skip(ctx):
        # Check if there are songs in the queue
        voice_client = ctx.voice_client
        if not play_queue:
            await ctx.send("The play queue is empty.")
            voice_client.stop()
            return

        # Get the next song URL from the queue
        next_song = play_queue.pop(0)

        with yt.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(next_song, download=False)
            url2 = info['url']
            voice_client.stop()
            voice_client.play(discord.FFmpegPCMAudio(url2))

        await ctx.send(f"Now playing: {info['title']}")

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

    @client.command()
    async def splay(ctx, query):
        async with semaphore:
            # Check if the bot is in a voice channel
            if ctx.voice_client is None:
                await ctx.send('I am not in a voice channel. Use `!join` to bring me into a channel.')
                return

            if ctx.voice_client.is_playing():
                pass

            # Use Spotipy to search for tracks based on the query
            results = sp.search(query, limit=1, type='track')

            # Check if any tracks were found
            if len(results['tracks']['items']) == 0:
                await ctx.send('No tracks found.')
                return
            with open('bot.txt', 'w') as save:
                save.write(str(results))

            # Get the first track found
            track_uri = results['tracks']['items'][0]['uri']

            # Get the audio stream URL for the Spotify track
            track_info = sp.track(track_uri)
            audio_url = track_info['preview_url']

            if audio_url is None:
                await ctx.send('The selected track does not have a preview available.')
                return

            # Play the audio URL in the voice channel
            ctx.voice_client.stop()
            ctx.voice_client.play(discord.FFmpegPCMAudio(audio_url))
            await ctx.send(f'Now playing: {results["tracks"]["items"][0]["name"]}')

    client.run(token)
