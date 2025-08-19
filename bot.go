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
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/lrstanley/go-ytdlp"
	"layeh.com/gopus"
)

var BotToken string
var voiceConnections = make(map[string]*discordgo.VoiceConnection)

type VoicePlayer struct {
	Playing bool
	VC      *discordgo.VoiceConnection
	Queue   []string
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
		discord.ChannelMessageSend(message.ChannelID, "Leaveing the voice channel")
	} else {
		discord.ChannelMessageSend(message.ChannelID, "I'm not in a voice channel")
	}
}

func PlayMusic(vc *discordgo.VoiceConnection, filePath string, discord *discordgo.Session, message *discordgo.MessageCreate) {
	players[vc.GuildID] = &VoicePlayer{
		Playing: true,
		VC:      vc,
	}
	vc.Speaking(true)
	ffmpeg := exec.Command("ffmpeg", "-i", filePath, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	ffmpegOut, erro := ffmpeg.StdoutPipe()
	if erro != nil {
		fmt.Println(erro)
		discord.ChannelMessageSend(message.ChannelID, "There was an error while trying to play the music")
	}
	err := ffmpeg.Start()
	if err != nil {
		fmt.Println("Error starting ffmpeg:", err)
		return
	}

	encoder, _ := gopus.NewEncoder(48000, 2, gopus.Audio)
	pcm := make([]int16, 960*2) // 20ms stereo

	for {
		err := binary.Read(ffmpegOut, binary.LittleEndian, pcm)
		if err != nil {
			break
		}
		opus, _ := encoder.Encode(pcm, 960, 960*2*2)
		vc.OpusSend <- opus
	}

	ffmpeg.Wait()
	players[vc.GuildID].Playing = false
}

func StopMusic(vc *discordgo.VoiceConnection, discord *discordgo.Session, message *discordgo.MessageCreate) {
	vc.Speaking(false)
	//close(vc.OpusSend)
	players[vc.GuildID].Playing = false
	discord.ChannelMessageSend(message.ChannelID, "Now stoping")
}

func IsPlaying(guildID string) bool {
	p, ok := players[guildID]
	return ok && p.Playing
}

func CheckIfCachedMusic(filepath string) bool {
	if _, err := os.Stat(filepath); os.IsExist(err) {
		fmt.Println("File already exists in cache")
		return true
	}
	return false
}

func DownlaodMusicFromLink(link string) (string, error) {
	//os.Setenv("YTDLP_DEBUG", "true")
	ytdlp.MustInstallAll(context.TODO())

	dl := ytdlp.New().
		PrintJSON().
		NoProgress().
		FormatSort("bestaudio").
		ExtractAudio().
		AudioFormat("mp3").
		Output("%(id)s.%(ext)s").
		ProgressFunc(100*time.Millisecond, func(prog ytdlp.ProgressUpdate) {
			fmt.Printf( //nolint:forbidigo
				"%s @ %s [eta: %s] :: %s\n",
				prog.Status,
				prog.PercentString(),
				prog.ETA(),
				prog.Filename,
			)
		})

	r, err := dl.Run(context.TODO(), link)
	if err != nil {
		return "", err
	}
	var data map[string]any
	erro := json.Unmarshal([]byte(r.Stdout), &data)
	if erro != nil {
		return "", erro
	}
	fmt.Println(data)
	if data["id"] != nil {
		id_numer, ok := data["id"].(string)
		if !ok {
			fmt.Println("There is and error while trying to conver id to string")
			return "", fmt.Errorf("There is and error while trying to conver id to string")
		}
		fmt.Println("stdout\n", data["id"])
		return id_numer + ".mp3", nil
	}
	return "", fmt.Errorf("filename not found")
}

func DownlaodMusicFromQuerry(querry string) (string, error) {
	ytdlp.MustInstallAll(context.TODO())

	query := fmt.Sprintf("ytsearch1:%s", querry)

	dl := ytdlp.New().
		PrintJSON().
		NoProgress().
		FormatSort("bestaudio").
		ExtractAudio().
		AudioFormat("mp3").
		Output("%(id)s.%(ext)s").
		ProgressFunc(100*time.Millisecond, func(prog ytdlp.ProgressUpdate) {
			fmt.Printf("%s @ %s [eta: %s] :: %s\n",
				prog.Status,
				prog.PercentString(),
				prog.ETA(),
				prog.Filename,
			)
		})

	r, err := dl.Run(context.TODO(), query)
	if err != nil {
		return "", err
	}
	var data map[string]any
	erro := json.Unmarshal([]byte(r.Stdout), &data)
	if erro != nil {
		return "", erro
	}
	if data["id"] != nil {
		id_numer, ok := data["id"].(string)
		if !ok {
			fmt.Println("There is and error while trying to conver id to string")
			return "", fmt.Errorf("There is and error while trying to conver id to string")
		}
		fmt.Println("stdout\n", data["id"])
		return id_numer + ".mp3", nil
	}
	return "", fmt.Errorf("filename not found")
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discord.State.User.ID {
		return
	}
	switch {
	case strings.Contains(message.Content, "!help"):
		PrintHelp(discord, message)
	case strings.Contains(message.Content, "!queue"):
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
	case strings.Contains(message.Content, "!play"):
		vs, err := findUserVoiceState(discord, message.GuildID, message.Author.ID)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, "You must be in a voice channel first!")
			return
		}

		parts := strings.Split(message.Content, " ")
		querry := strings.Join(parts[1:], " ")

		// Ensure bot joins if not already
		if _, ok := voiceConnections[vs.GuildID]; !ok {
			JoinServer(discord, message)
		}

		var filename string
		var erro error

		if len(parts) > 2 {
			// Search query
			filename, erro = DownlaodMusicFromQuerry(querry)
		} else if len(parts) == 2 {
			// Direct link
			filename, erro = DownlaodMusicFromLink(querry)
		}

		if erro != nil {
			discord.ChannelMessageSend(message.ChannelID, "Failed to download music: "+erro.Error())
			return
		}

		player, ok := players[vs.GuildID]
		if !ok {
			player = &VoicePlayer{VC: voiceConnections[vs.GuildID], Queue: []string{}}
			players[vs.GuildID] = player
		}

		if player.Playing {
			// Add to queue
			player.Queue = append(player.Queue, filename)
			discord.ChannelMessageSend(message.ChannelID, "Added to queue: "+filename)
		} else {
			// Play immediately
			go PlayMusic(player, filename, discord, message)
		}
	case strings.Contains(message.Content, "!stop"):
		StopMusic(voiceConnections[message.GuildID], discord, message)
	case strings.Contains(message.Content, "!leave"):
		LeaveServer(discord, message)
	}
}

func main() {
	Run()
}
