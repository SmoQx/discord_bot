package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/lrstanley/go-ytdlp"
	"layeh.com/gopus"
)

var BotToken string
var voiceConnections = make(map[string]*discordgo.VoiceConnection)

type Config struct {
	BotToken string
}

type Song struct {
	Title    string
	Filename string
}

type VoicePlayer struct {
	Playing     bool
	VC          *discordgo.VoiceConnection
	Queue       []Song
	FFmpegCmd   *exec.Cmd
	AutoAdvance bool
}

var players = make(map[string]*VoicePlayer)

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Something whent wrong")
	}
}

func findUserVoiceState(s *discordgo.Session, guildID, userID string) (*discordgo.VoiceState, error) {
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return nil, err
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs, nil
		}
	}
	return nil, fmt.Errorf("user not in a voice channel")
}

func Run() {
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	discord.AddHandler(newMessage)
	discord.AddHandler(voiceStateUpdate)

	discord.Open()
	defer discord.Close()

	fmt.Println("Bot started")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

}

func voiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// Check if the bot is connected in this guild
	vc, ok := voiceConnections[vs.GuildID]
	if !ok || vc == nil {
		return
	}

	// Get the channel the bot is in
	botChannelID := vc.ChannelID
	if botChannelID == "" {
		return
	}

	// Count members in the bot's voice channel
	guild, err := s.State.Guild(vs.GuildID)
	if err != nil {
		return
	}

	memberCount := 0
	for _, state := range guild.VoiceStates {
		if state.ChannelID == botChannelID && state.UserID != s.State.User.ID {
			memberCount++
		}
	}

	// If no one else is left, disconnect
	if memberCount == 0 {
		fmt.Println("No one left, leaving channel.")
		vc.Disconnect()
		delete(voiceConnections, vs.GuildID)
		delete(players, vs.GuildID)
	}
	fmt.Println("memeber count :", memberCount, "voice channel bot id", botChannelID)
}

func PrintHelp(discord *discordgo.Session, message *discordgo.MessageCreate) {
	message_text := `This message is shown when you need help:
!queue - list ququeue
!join - bot joins
!skip - skips current song
!stop - stops the bot
!leave - bot leaves
!help - showes this message`
	discord.ChannelMessageSend(message.ChannelID, message_text)
}

func JoinServer(discord *discordgo.Session, message *discordgo.MessageCreate) {
	// Find the voice state for the user in the guild
	vs, err := findUserVoiceState(discord, message.GuildID, message.Author.ID)
	if err != nil {
		discord.ChannelMessageSend(message.ChannelID, "You must be in a voice channel first!")
		return
	}

	// Connect to that voice channel
	vc, err := discord.ChannelVoiceJoin(message.GuildID, vs.ChannelID, false, true)
	if err != nil {
		discord.ChannelMessageSend(message.ChannelID, "Failed to join voice channel.")
		fmt.Println("Error joining voice channel:", err)
		return
	}

	voiceConnections[message.GuildID] = vc
	discord.ChannelMessageSend(message.ChannelID, "Joined your voice channel!")
}

func LeaveServer(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if vc, ok := discord.VoiceConnections[message.GuildID]; ok {
		vc.Disconnect()
		delete(voiceConnections, vc.GuildID)
		delete(players, vc.GuildID)
		discord.ChannelMessageSend(message.ChannelID, "Leaveing the voice channel")
	} else {
		discord.ChannelMessageSend(message.ChannelID, "I'm not in a voice channel")
	}
}

func PlayMusic(player *VoicePlayer, song Song, discord *discordgo.Session, message *discordgo.MessageCreate) {
	// Start a fresh playback: allow auto-advance unless a skip/stop disables it
	player.Playing = true
	player.AutoAdvance = true

	vc := player.VC
	discord.ChannelMessageSend(message.ChannelID, "Now playing: **"+song.Title+"**")

	vc.Speaking(true)
	ffmpeg := exec.Command("ffmpeg", "-i", "./cache/"+song.Filename, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	player.FFmpegCmd = ffmpeg

	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		fmt.Println("ffmpeg StdoutPipe error:", err)
		discord.ChannelMessageSend(message.ChannelID, "Error starting audio pipeline.")
		return
	}
	if err := ffmpeg.Start(); err != nil {
		fmt.Println("Error starting ffmpeg:", err)
		discord.ChannelMessageSend(message.ChannelID, "Error starting ffmpeg.")
		return
	}

	encoder, _ := gopus.NewEncoder(48000, 2, gopus.Audio)
	pcm := make([]int16, 960*2) // 20ms stereo

	for {
		if err := binary.Read(ffmpegOut, binary.LittleEndian, pcm); err != nil {
			break
		}
		opus, _ := encoder.Encode(pcm, 960, 960*2*2)
		vc.OpusSend <- opus

		if !player.Playing { // stop/skip requested
			break
		}
	}

	// Tear down this playback
	_ = ffmpeg.Wait()
	vc.Speaking(false)
	player.FFmpegCmd = nil
	player.Playing = false

	if player.AutoAdvance && len(player.Queue) > 0 {
		next := player.Queue[0]
		player.Queue = player.Queue[1:]
		go PlayMusic(player, next, discord, message)
	} else if !player.AutoAdvance {
		// skip/stop handled the next step explicitly
	} else {
		discord.ChannelMessageSend(message.ChannelID, "Queue finished.")
	}
}

func StopMusic(vc *discordgo.VoiceConnection, discord *discordgo.Session, message *discordgo.MessageCreate) {
	player, ok := players[vc.GuildID]
	if !ok || !player.Playing {
		discord.ChannelMessageSend(message.ChannelID, "No music is currently playing.")
		return
	}

	player.AutoAdvance = false
	player.Playing = false

	if player.FFmpegCmd != nil {
		_ = player.FFmpegCmd.Process.Kill()
		player.FFmpegCmd = nil
	}

	player.Queue = []Song{}
	vc.Speaking(false)
	discord.ChannelMessageSend(message.ChannelID, "Stopped playback and cleared the queue.")
}

func IsPlaying(guildID string) bool {
	p, ok := players[guildID]
	return ok && p.Playing
}

func CheckIfCachedMusic(filepath string) bool {
	if _, err := os.Stat(filepath); err == nil {
		fmt.Println("File already exists in cache:", filepath)
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		fmt.Println("Error checking file:", err)
		return false
	}
}

func DownlaodMusicFromLink(link string) (Song, error) {
	ytdlp.MustInstallAll(context.TODO())

	dl := ytdlp.New().
		PrintJSON().
		NoPlaylist().
		NoProgress().
		FormatSort("bestaudio").
		ExtractAudio().
		AudioFormat("mp3").
		Output("./cache/%(id)s.%(ext)s")

	r, err := dl.Run(context.TODO(), link)
	if err != nil {
		return Song{}, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return Song{}, err
	}

	id, _ := data["id"].(string)
	title, _ := data["title"].(string)

	return Song{
		Title:    title,
		Filename: id + ".mp3",
	}, nil
}

func DownlaodMusicFromQuerry(querry string) (Song, error) {
	ytdlp.MustInstallAll(context.TODO())

	query := fmt.Sprintf("ytsearch1:%s", querry)
	dl := ytdlp.New().
		PrintJSON().
		NoPlaylist().
		NoProgress().
		FormatSort("bestaudio").
		ExtractAudio().
		AudioFormat("mp3").
		Output("./cache/%(id)s.%(ext)s")

	r, err := dl.Run(context.TODO(), query)
	if err != nil {
		return Song{}, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return Song{}, err
	}

	id, _ := data["id"].(string)
	title, _ := data["title"].(string)

	return Song{
		Title:    title,
		Filename: id + ".mp3",
	}, nil
}

func SkipMusic(vc *discordgo.VoiceConnection, discord *discordgo.Session, message *discordgo.MessageCreate) {
	player, ok := players[vc.GuildID]
	if !ok || !player.Playing {
		discord.ChannelMessageSend(message.ChannelID, "No music is currently playing.")
		return
	}

	// Prevent the current PlayMusic from auto-advancing
	player.AutoAdvance = false
	player.Playing = false

	if player.FFmpegCmd != nil {
		_ = player.FFmpegCmd.Process.Kill()
		player.FFmpegCmd = nil
	}

	if len(player.Queue) > 0 {
		next := player.Queue[0]
		player.Queue = player.Queue[1:]
		discord.ChannelMessageSend(message.ChannelID, "Skipping… Now playing: **"+next.Title+"**")
		go PlayMusic(player, next, discord, message) // this new PlayMusic will reset AutoAdvance=true
	} else {
		discord.ChannelMessageSend(message.ChannelID, "Skipped. No more songs in the queue.")
	}
}

func GetVideoIDFromLink(link string) (Song, error) {
	ytdlp.MustInstallAll(context.TODO())

	dl := ytdlp.New().
		PrintJSON().
		NoProgress()

	r, err := dl.Run(context.TODO(), link)
	if err != nil {
		return Song{}, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return Song{}, err
	}

	id, _ := data["id"].(string)
	title, _ := data["title"].(string)

	return Song{
		Title:    title,
		Filename: id + ".mp3",
	}, nil
}

func GetVideoIDFromQuerry(query string) (Song, error) {
	ytdlp.MustInstallAll(context.TODO())

	searchQuery := fmt.Sprintf("ytsearch1:%s", query)

	dl := ytdlp.New().
		PrintJSON().
		NoProgress()

	r, err := dl.Run(context.TODO(), searchQuery)
	if err != nil {
		return Song{}, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return Song{}, err
	}

	id, _ := data["id"].(string)
	title, _ := data["title"].(string)

	return Song{
		Title:    title,
		Filename: id + ".mp3",
	}, nil
}

func IsPlaylist(link string) (bool, error) {
	ytdlp.MustInstallAll(context.TODO())

	dl := ytdlp.New().
		PrintJSON().
		LazyPlaylist() // ✅ only fetch high-level info, not all videos

	r, err := dl.Run(context.TODO(), link)
	if err != nil {
		return false, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return false, err
	}

	// If "entries" key exists → it's a playlist
	if _, ok := data["entries"]; ok {
		return true, nil
	}

	return false, nil
}

func FetchPlaylistEntries(link string) ([]string, error) {
	ytdlp.MustInstallAll(context.TODO())

	r, err := ytdlp.New().
		PrintJSON().
		DumpSingleJSON().
		Run(context.TODO(), link)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &data); err != nil {
		return nil, err
	}

	entries, ok := data["entries"].([]any)
	if !ok {
		return nil, fmt.Errorf("not a playlist or missing entries")
	}

	urls := make([]string, 0, len(entries))
	for _, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		url, _ := entry["webpage_url"].(string)
		if url != "" {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

func DownloadPlaylist(urls []string, player *VoicePlayer, discord *discordgo.Session, message *discordgo.MessageCreate) {
	go func() {
		for i, url := range urls {
			song, err := DownlaodMusicFromLink(url)
			if err != nil {
				discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Failed to download: %s", song.Title))
				continue
			}

			// If it's the first track and nothing is playing → start
			if i == 0 && !player.Playing {
				go PlayMusic(player, song, discord, message)
			} else {
				// just enqueue the song
				player.Queue = append(player.Queue, song)
			}
		}
	}()
}

func showQueue(player *VoicePlayer) string {
	if len(player.Queue) == 0 {
		return "Queue is empty."
	}

	var titles []string
	for _, song := range player.Queue {
		titles = append(titles, song.Title)
	}

	return strings.Join(titles, "\n") // join titles with newlines
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discord.State.User.ID {
		return
	}
	switch {
	case strings.Contains(message.Content, "!help"):
		PrintHelp(discord, message)
	case strings.Contains(message.Content, "!queue"):
		vs, err := findUserVoiceState(discord, message.GuildID, message.Author.ID)
		if err != nil {
			return
		}
		player, _ := players[vs.GuildID]
		discord.ChannelMessageSend(message.ChannelID, showQueue(player))
	case strings.Contains(message.Content, "!download_q"):
		parts := strings.Split(message.Content, " ")
		querry := strings.Join(parts[1:], " ")
		DownlaodMusicFromQuerry(querry)
	case strings.Contains(message.Content, "!download"):
		fmt.Println(message.Content)
		parts := strings.Split(message.Content, " ")
		fmt.Println(parts)
		fmt.Println(parts[1])
		DownlaodMusicFromLink(parts[1])
	case strings.Contains(message.Content, "!join"):
		JoinServer(discord, message)
	case strings.Contains(message.Content, "!skip"):
		if vc, ok := voiceConnections[message.GuildID]; ok {
			SkipMusic(vc, discord, message)
		} else {
			discord.ChannelMessageSend(message.ChannelID, "I'm not in a voice channel.")
		}
	case strings.Contains(message.Content, "!play"):
		vs, err := findUserVoiceState(discord, message.GuildID, message.Author.ID)
		parts := strings.Split(message.Content, " ")
		query := strings.Join(parts[1:], " ")

		if _, ok := voiceConnections[vs.GuildID]; !ok {
			JoinServer(discord, message)
		}

		isPl, _ := IsPlaylist(query)
		if isPl {
			fmt.Println("playlist detected")
			urls, err := FetchPlaylistEntries(query)
			if err != nil {
				discord.ChannelMessageSend(message.ChannelID, "Failed to fetch playlist entries: "+err.Error())
				return
			}

			player, ok := players[vs.GuildID]
			if !ok {
				player = &VoicePlayer{
					VC:          voiceConnections[vs.GuildID],
					Queue:       []Song{},
					AutoAdvance: true,
				}
				players[vs.GuildID] = player
			}

			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Playlist detected with %d songs. Starting download...", len(urls)))
			DownloadPlaylist(urls, player, discord, message)
			return
		}

		// Download song (either by search or link)
		var song Song
		if len(parts) > 2 {
			song, err = GetVideoIDFromQuerry(query)
			if !CheckIfCachedMusic(song.Filename) {
				discord.ChannelMessageSend(message.ChannelID, "Downloading started")
				song, err = DownlaodMusicFromQuerry(query)
				discord.ChannelMessageSend(message.ChannelID, "Downloaded"+song.Title)
			}
		} else if len(parts) == 2 {
			song, err = GetVideoIDFromLink(query)
			if !CheckIfCachedMusic(song.Filename) {
				discord.ChannelMessageSend(message.ChannelID, "Downloading started")
				song, err = DownlaodMusicFromLink(query)
				discord.ChannelMessageSend(message.ChannelID, "Downloaded"+song.Title)
			}
		}
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, "Failed to download: "+err.Error())
			return
		}

		player, ok := players[vs.GuildID]
		if !ok {
			player = &VoicePlayer{
				VC:          voiceConnections[vs.GuildID],
				Queue:       []Song{},
				AutoAdvance: true,
			}
			players[vs.GuildID] = player
		}

		// If playing, add to queue, otherwise play immediately
		if player.Playing {
			player.Queue = append(player.Queue, song)
			discord.ChannelMessageSend(message.ChannelID, "Added to queue: **"+song.Title+"**")
		} else {
			go PlayMusic(player, song, discord, message)
		}
	case strings.Contains(message.Content, "!stop"):
		StopMusic(voiceConnections[message.GuildID], discord, message)
	case strings.Contains(message.Content, "!leave"):
		StopMusic(voiceConnections[message.GuildID], discord, message)
		LeaveServer(discord, message)
	}
}

func main() {

	bytes, err := os.ReadFile("config.json") // replaces ioutil.ReadFile
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	// Unmarshal JSON
	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		log.Fatal("Error decoding JSON:", err)
	}

	BotToken = config.BotToken

	Run()
}
