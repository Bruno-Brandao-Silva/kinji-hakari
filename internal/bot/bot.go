package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"

	"hakari-bot/internal/audio"
)

const (
	voiceOpenTimeout  = 15 * time.Second
	voiceCloseTimeout = 10 * time.Second
	aloneLeaveDelay   = 5 * time.Second
	// Lets the sender flush trailing silence frames before disconnecting.
	finishLeaveDelay = time.Second
)

// session is the bot's playback state in one guild. channelID tells stale
// voice events (e.g. leaving the previous channel while switching) apart from current ones.
type session struct {
	player    *audio.Player
	channelID snowflake.ID
}

// Bot handles commands and voice events. Reconnection and voice server
// migration are handled by disgo's voice connection itself.
type Bot struct {
	track *audio.Track

	// connectMu serializes connection changes so concurrent /jackpot calls cannot open duplicate connections.
	connectMu sync.Mutex
	mu        sync.Mutex
	sessions  map[snowflake.ID]*session
}

func New(track *audio.Track) *Bot {
	return &Bot{track: track, sessions: make(map[snowflake.ID]*session)}
}

// OnReady logs the authenticated bot user.
func (b *Bot) OnReady(e *events.Ready) {
	slog.Info("Bot logado", "user", e.User.Username)
}

func (b *Bot) play(client *disgobot.Client, guildID, channelID snowflake.ID, loops, volume int) error {
	b.connectMu.Lock()
	defer b.connectMu.Unlock()

	var player *audio.Player
	player, err := audio.NewPlayer(b.track, loops, volume, func() { b.finish(client, guildID, player) })
	if err != nil {
		return err
	}

	b.setSession(guildID, &session{player: player, channelID: channelID})
	conn, err := connect(client, guildID, channelID)
	if err != nil {
		b.removeSession(guildID, player)
		return err
	}
	conn.SetOpusFrameProvider(player)
	return nil
}

// connect reuses the guild connection when already in channelID and reopens it otherwise.
func connect(client *disgobot.Client, guildID, channelID snowflake.ID) (voice.Conn, error) {
	conn := client.VoiceManager.GetConn(guildID)
	if conn != nil {
		if current := conn.ChannelID(); current != nil && *current == channelID {
			return conn, nil
		}
		slog.Info("Mudando de canal", "guild_id", guildID, "new_channel", channelID)
		closeConn(conn)
	}

	slog.Info("Conectando ao canal de voz...", "guild_id", guildID, "channel_id", channelID)
	conn = client.VoiceManager.CreateConn(guildID)
	ctx, cancel := context.WithTimeout(context.Background(), voiceOpenTimeout)
	defer cancel()
	if err := conn.Open(ctx, channelID, false, true); err != nil {
		closeConn(conn)
		return nil, fmt.Errorf("open voice connection: %w", err)
	}
	return conn, nil
}

// finish leaves the channel when player is still the guild's active player.
func (b *Bot) finish(client *disgobot.Client, guildID snowflake.ID, player *audio.Player) {
	time.Sleep(finishLeaveDelay)
	if b.removeSession(guildID, player) {
		slog.Info("Playback finalizado, saindo do canal", "guild_id", guildID)
		closeConn(client.VoiceManager.GetConn(guildID))
	}
}

func (b *Bot) leave(client *disgobot.Client, guildID snowflake.ID) {
	b.mu.Lock()
	delete(b.sessions, guildID)
	b.mu.Unlock()
	closeConn(client.VoiceManager.GetConn(guildID))
}

func closeConn(conn voice.Conn) {
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), voiceCloseTimeout)
	defer cancel()
	conn.Close(ctx)
}

func (b *Bot) currentPlayer(guildID snowflake.ID) *audio.Player {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[guildID]; ok {
		return s.player
	}
	return nil
}

func (b *Bot) setSession(guildID snowflake.ID, s *session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessions[guildID] = s
}

// removeSession deletes the guild session only if it still belongs to player.
func (b *Bot) removeSession(guildID snowflake.ID, player *audio.Player) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.sessions[guildID]; ok && s.player == player {
		delete(b.sessions, guildID)
		return true
	}
	return false
}

// sessionInChannel reports whether the guild session is in channelID.
func (b *Bot) sessionInChannel(guildID snowflake.ID, channelID *snowflake.ID) bool {
	if channelID == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.sessions[guildID]
	return ok && s.channelID == *channelID
}
