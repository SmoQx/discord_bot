import discord
import responses
from discord.ext import commands
import youtube_dl


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

    @client.command()
    async def play(ctx, url):
        ydl_opts = {
            'format': 'bestaudio/best',
            'postprocessors': [{
                'key': 'FFmpegExtractAudio',
                'preferredcodec': 'mp3',
                'preferredquality': '192',
            }],
        }

        with youtube_dl.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
            url2 = info['formats'][0]['url']
            voice_client = ctx.voice_client
            voice_client.stop()
            voice_client.play(discord.FFmpegPCMAudio(url2))

    @client.event
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
                print(message)

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
