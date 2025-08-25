package main

import (
	"context"
	"database/sql"
	"discord_bot/crud"
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

func Run(token string, db *sql.DB) {
	discord, err := discordgo.New("Bot " + token)
	checkNilErr(err)

	// discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 	newMessage(s, m, db) // pass db yourself
	// })

	discord.AddHandler(voiceStateUpdate)

	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		newCommand(s, i, db)
	})
	discord.Open()

	defer discord.Close()

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "play",
			Description: "Play a song from a link or search query",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "query",
					Description: "YouTube URL or search query",
					Required:    true,
				},
			},
		},
		{
			Name:        "join",
			Description: "Join the server",
		},
		{
			Name:        "stop",
			Description: "Stop playback and clear the queue",
		},
		{
			Name:        "leave",
			Description: "Stop playback, clear the queue and leave",
		},
		{
			Name:        "stats",
			Description: "Server music stats",
		},
		{
			Name:        "help",
			Description: "Shows all commands which are avaiable on this server",
		},
		{
			Name:        "queue",
			Description: "Shows current queue",
		},
		{
			Name:        "skip",
			Description: "Skips currently playing song",
		},
	}

	for _, cmd := range commands {
		_, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", cmd)
		if err != nil {
			log.Fatalf("Cannot create '%v' command: %v", cmd.Name, err)
		}
	}
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
!help - showes this message
!stats - for servers songs statistics`
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

func JoinServerFromCommand(discord *discordgo.Session, i *discordgo.InteractionCreate) {
	// Find the voice state for the user in the guild
	vs, err := findUserVoiceState(discord, i.GuildID, i.Member.User.ID)
	if err != nil {
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "You must be in a voice channel first!",
		})
		return
	}

	// Connect to that voice channel
	vc, err := discord.ChannelVoiceJoin(i.GuildID, vs.ChannelID, false, true)
	if err != nil {
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Failed to join voice channel.",
		})
		fmt.Println("Error joining voice channel:", err)
		return
	}

	voiceConnections[i.GuildID] = vc
	discord.ChannelMessageSend(i.ChannelID, "Joined your voice channel!")
}

func LeaveServerForInteraction(discord *discordgo.Session, i *discordgo.InteractionCreate) {
	if vc, ok := discord.VoiceConnections[i.GuildID]; ok {
		vc.Disconnect()
		delete(voiceConnections, vc.GuildID)
		delete(players, vc.GuildID)
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Leaveing the voice channel",
		})
	} else {
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "I'm not in a voice channel",
		})
	}
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

func PlayMusicFromInteraction(player *VoicePlayer, song Song, discord *discordgo.Session, i *discordgo.InteractionCreate) {
	// Start a fresh playback: allow auto-advance unless a skip/stop disables it
	player.Playing = true
	player.AutoAdvance = true

	vc := player.VC
	discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: "Now playing: **" + song.Title + "**",
	})

	vc.Speaking(true)
	ffmpeg := exec.Command("ffmpeg", "-i", "./cache/"+song.Filename, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	player.FFmpegCmd = ffmpeg

	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		fmt.Println("ffmpeg StdoutPipe error:", err)
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Error starting audio pipeline.",
		})
		return
	}
	if err := ffmpeg.Start(); err != nil {
		fmt.Println("Error starting ffmpeg:", err)
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Error starting ffmpeg.",
		})
		return
	}

	discord.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: "online",
		Activities: []*discordgo.Activity{
			{
				Name: song.Title,
				Type: discordgo.ActivityTypeListening,
			},
		},
	})

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
		go PlayMusicFromInteraction(player, next, discord, i)
	} else if !player.AutoAdvance {
		// skip/stop handled the next step explicitly
	} else {
		discord.UpdateStatusComplex(discordgo.UpdateStatusData{
			Status:     "online",
			Activities: []*discordgo.Activity{},
		})
		discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Content: "Queue finished.",
		})
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

func StopMusicForInteraction(vc *discordgo.VoiceConnection, discord *discordgo.Session, interaction *discordgo.InteractionCreate) {
	player, ok := players[vc.GuildID]
	if !ok || !player.Playing {
		discord.FollowupMessageCreate(interaction.Interaction, false, &discordgo.WebhookParams{
			Content: "No music is currently playing.",
		})
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
	discord.FollowupMessageCreate(interaction.Interaction, false, &discordgo.WebhookParams{
		Content: "Stopped playback and cleared the queue.",
	})
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
	if filepath == "" {
		fmt.Println("got an empty string")
		return false
	}
	if _, err := os.Stat("./cache/" + filepath); err == nil {
		fmt.Println("File already exists in cache:", "./cache/"+filepath)
		return true
	} else if os.IsNotExist(err) {
		fmt.Println("File does not already exists in cache:", "./cache/"+filepath)
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

func SkipMusicForInteraction(vc *discordgo.VoiceConnection, discord *discordgo.Session, message *discordgo.InteractionCreate) {
	player, ok := players[vc.GuildID]
	if !ok || !player.Playing {
		discord.InteractionRespond(message.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No music is currently playing.",
			},
		})
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
		discord.InteractionRespond(message.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Skipping… Now playing: **" + next.Title + "**",
			},
		})
		go PlayMusicFromInteraction(player, next, discord, message) // this new PlayMusic will reset AutoAdvance=true
	} else {
		discord.InteractionRespond(message.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Skipped. No more songs in the queue.",
			},
		})
	}
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
		NoProgress().
		SkipDownload()

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
		NoProgress().
		SkipDownload()

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
		SkipDownload()

	r, err := dl.Run(context.TODO(), link)
	if err != nil {
		return false, err
	}

	var entries []map[string]interface{}
	// var data map[string]any
	lines := strings.Split(r.Stdout, "\n")
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Println("Error unmarshalling line:", err)
			continue
		}
		fmt.Println("Entry title:", entry["title"])
		entries = append(entries, entry)
	}

	if len(entries) > 1 {
		return true, nil
	}

	return false, nil
}

func FetchPlaylistEntries(link string) ([]string, error) {
	ytdlp.MustInstallAll(context.TODO())

	r, err := ytdlp.New().
		PrintJSON().
		DumpSingleJSON().
		SkipDownload().
		Run(context.TODO(), link)

	if err != nil {
		return nil, err
	}

	// var entries []map[string]interface{}
	var urls []string
	lines := strings.Split(r.Stdout, "\n")
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Println("Error unmarshalling line:", err)
			continue
		}
		fmt.Println("Entry title:", entry["title"])

		// Safe type assertion
		if urlVal, ok := entry["webpage_url"].(string); ok && urlVal != "" {
			urls = append(urls, urlVal)
			fmt.Println("Entry URL:", urlVal)
		} else {
			fmt.Println("Entry missing URL, skipping")
		}
	}

	fmt.Println("All URLs:", urls)

	return urls[:len(urls)-1], nil
}

func DownloadPlaylistFromInteractoin(urls []string, player *VoicePlayer, discord *discordgo.Session, interaction *discordgo.InteractionCreate) {
	go func() {
		for i, url := range urls {
			song, err := DownlaodMusicFromLink(url)
			if err != nil {
				fmt.Println("Failed to download a playlist entry")
				discord.FollowupMessageCreate(interaction.Interaction, false, &discordgo.WebhookParams{
					Content: fmt.Sprintf("Failed to download: %s", song.Title),
				})
				continue
			}

			// If it's the first track and nothing is playing → start
			if i == 0 && !player.Playing {
				go PlayMusicFromInteraction(player, song, discord, interaction)
			} else {
				// just enqueue the song
				player.Queue = append(player.Queue, song)
			}
		}
	}()
}

func DownloadPlaylist(urls []string, player *VoicePlayer, discord *discordgo.Session, message *discordgo.MessageCreate) {
	go func() {
		for i, url := range urls {
			song, err := DownlaodMusicFromLink(url)
			if err != nil {
				fmt.Println("Failed to download a playlist entry")
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

func ShowPlayStatsForInteraction(discord *discordgo.Session, message *discordgo.InteractionCreate, db *sql.DB) {
	var songs []crud.Song_counter
	var err error
	songs, err = crud.ReadAllPlayedCountForSongInServer(message.GuildID, db)
	if err != nil {
		fmt.Println("Error while reading from database :", err)
	}
	fmt.Println(songs)

	var sb strings.Builder
	sb.WriteString("Songs statistics are:\n")

	for _, song := range songs {
		sb.WriteString(fmt.Sprintf("Song title %s was played ** %d **\n", song.Title, song.Played_counter))
		fmt.Println(song.Title, song.Played_counter)
	}
	sb.WriteString("Thats it folks")

	result := sb.String()
	fmt.Println(result)

	discord.FollowupMessageCreate(message.Interaction, false, &discordgo.WebhookParams{
		Content: result,
	})
}

func ShowPlayStats(discord *discordgo.Session, message *discordgo.MessageCreate, db *sql.DB) {
	var songs []crud.Song_counter
	var err error
	songs, err = crud.ReadAllPlayedCountForSongInServer(message.GuildID, db)
	if err != nil {
		fmt.Println("Error while reading from database :", err)
	}
	fmt.Println(songs)

	var sb strings.Builder
	sb.WriteString("Songs statistics are:\n")

	for _, song := range songs {
		sb.WriteString(fmt.Sprintf("Song title %s was played %d\n", song.Title, song.Played_counter))
		fmt.Println(song.Title, song.Played_counter)
	}
	sb.WriteString("Thats it folks")

	result := sb.String()
	fmt.Println(result)

	discord.ChannelMessageSend(message.ChannelID, result)
}

func newCommand(discord *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB) {
	user_id := i.Member.User.ID
	username := i.Member.User.Username
	crud.InsertUserIntoDatabase(username, user_id, db)
	if i.Type == discordgo.InteractionApplicationCommand {
		switch i.ApplicationCommandData().Name {
		case "help":
			message_text := `This message is shown when you need help:
!queue - list ququeue
!join - bot joins
!skip - skips current song
!stop - stops the bot
!leave - bot leaves
!help - showes this message
!stats - for servers songs statistics`
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message_text,
				},
			})
		case "queue":
			vs, err := findUserVoiceState(discord, i.GuildID, i.Member.User.ID)
			if err != nil {
				return
			}
			player, ok := players[vs.GuildID]
			if !ok {
				return
			}
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Current playlist " + "**" + showQueue(player) + "**",
				},
			})
		case "stats":
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Current statistics:",
				},
			})
			ShowPlayStatsForInteraction(discord, i, db)
		case "stop":
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Stoping!",
				},
			})
			StopMusicForInteraction(voiceConnections[i.GuildID], discord, i)
		case "skip":
			if vc, ok := voiceConnections[i.GuildID]; ok {
				SkipMusicForInteraction(vc, discord, i)
			} else {
				discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "I'm not in a voice channel.",
					},
				})
			}
		case "leave":
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Leave Server!",
				},
			})
			StopMusicForInteraction(voiceConnections[i.GuildID], discord, i)
			LeaveServerForInteraction(discord, i)
		case "join":
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Joining the server",
				},
			})
			JoinServerFromCommand(discord, i)
		case "play":
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "I will start the music soon ",
				},
			})
			query := i.ApplicationCommandData().Options[0].StringValue()
			// here you can call your existing !play logic, reusing PlayMusic

			vs, err := findUserVoiceState(discord, i.GuildID, i.Member.User.ID)

			if _, ok := voiceConnections[vs.GuildID]; !ok {
				JoinServerFromCommand(discord, i)
			}

			// Download song (either by search or link)
			var song Song
			if !strings.Contains(query, "http") {
				song, err = GetVideoIDFromQuerry(query)
				if !CheckIfCachedMusic(song.Filename) {
					discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: "Downloading started",
					})
					song, err = DownlaodMusicFromQuerry(query)
					discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: "Downloaded" + song.Title,
					})
				}
			} else if strings.Contains(query, "http") {
				isPl, playlist_error := IsPlaylist(query)
				fmt.Println(playlist_error)
				if isPl {
					fmt.Println("Playlist detected")
					urls, err := FetchPlaylistEntries(query)
					if err != nil {
						discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
							Content: "Failed to fetch playlist entries: " + err.Error(),
						})
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

					discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
						Content: fmt.Sprintf("Playlist detected with %d songs. Starting download...", len(urls)),
					})
					DownloadPlaylistFromInteractoin(urls, player, discord, i)
					return
				} else {
					song, err = GetVideoIDFromLink(query)
					if !CheckIfCachedMusic(song.Filename) {
						discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
							Content: "Downloading started",
						})
						song, err = DownlaodMusicFromLink(query)
						discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
							Content: "Downloaded" + song.Title,
						})
					}
				}
			}
			crud.InsertSongIntoDatabase(song.Filename, song.Title, i.GuildID, db)
			crud.UpdateSongsPlayCount(song.Filename, i.GuildID, db)
			if err != nil {
				discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: "Failed to download: " + err.Error(),
				})
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
				discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: "Added to queue: **" + song.Title + "**",
				})
			} else {
				go PlayMusicFromInteraction(player, song, discord, i)
			}
		}
	}
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate, db *sql.DB) {
	if message.Author.ID == discord.State.User.ID {
		return
	}
	user_id := message.Author.ID
	username := message.Author.Username
	crud.InsertUserIntoDatabase(username, user_id, db)
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
	case strings.Contains(message.Content, "!stats"):
		ShowPlayStats(discord, message, db)
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

		// Download song (either by search or link)
		var song Song
		if len(parts) >= 2 && !strings.Contains(query, "http") {
			song, err = GetVideoIDFromQuerry(query)
			if !CheckIfCachedMusic(song.Filename) {
				discord.ChannelMessageSend(message.ChannelID, "Downloading started")
				song, err = DownlaodMusicFromQuerry(query)
				discord.ChannelMessageSend(message.ChannelID, "Downloaded"+song.Title)
			}
		} else if len(parts) == 2 && strings.Contains(query, "http") {
			isPl, playlist_error := IsPlaylist(query)
			fmt.Println(playlist_error)
			if isPl {
				fmt.Println("Playlist detected")
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
			} else {
				song, err = GetVideoIDFromLink(query)
				if !CheckIfCachedMusic(song.Filename) {
					discord.ChannelMessageSend(message.ChannelID, "Downloading started")
					song, err = DownlaodMusicFromLink(query)
					discord.ChannelMessageSend(message.ChannelID, "Downloaded"+song.Title)
				}
			}
		}
		crud.InsertSongIntoDatabase(song.Filename, song.Title, message.GuildID, db)
		crud.UpdateSongsPlayCount(song.Filename, message.GuildID, db)
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

func MainBOT(db *sql.DB) {

	bytes, err := os.ReadFile("conf.json") // replaces ioutil.ReadFile
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	// Unmarshal JSON
	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		log.Fatal("Error decoding JSON:", err)
	}

	Run(config.BotToken, db)
}
