import discord
import responses
from discord.ext import commands
import yt_dlp as yt
import asyncio


async def send_message(message, user_message, is_private):
    try:
        response = responses.response_message(user_message)
        await message.author.send(response) if is_private else await message.channel.send(response)
    except Exception as e:
        print(e)


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
        await ctx.send(f'Songs in q: \n {title}')

    @client.command()
    async def play(ctx, url):
        play_queue.append(url)
        channel = ctx.author.voice.channel

        if ctx.voice_client is None:
            await channel.connect()
        else:
            print("already connected")

        if not ctx.voice_client.is_playing():
            await play_next(ctx)
        else:
            with yt.YoutubeDL(ydl_opts) as ydl:
                info = ydl.extract_info(url, download=False)
            await ctx.send(f"Added to q: {info['title']}.")

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

    """@client.event
    async def on_ready():
        print(f'{client.user} is now running!')

    @client.event
    async def on_message(message):
        if message.author == client.user:
            return
        if message.author.bot:
            return

        username = str(message.author)
        user_message = str(message.content)
        channel = str(message.channel)

        print(f'{username}, said {user_message}, on channel {channel}')
        if user_message[0] == '?':
            user_message = user_message[1:]
            await send_message(message, user_message, is_private=True)
        else:
            await send_message(message, user_message, is_private=False)
            if responses.response_message(user_message) == 'Now playing! ':
                print(message)"""

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
