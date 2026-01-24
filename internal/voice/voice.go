package voice

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

const (
	channels  = 2
	frameRate = 48000
	frameSize = 960
	maxBytes  = 4000
)

type Session struct {
	GuildID        string
	ChannelID      string
	Connection     *discordgo.VoiceConnection
	Cancel         context.CancelFunc
	DiscordSession *discordgo.Session // Reference for reconnection
	LazyExit       bool
	mu             sync.Mutex
}

type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var GlobalManager = &Manager{
	sessions: make(map[string]*Session),
}

func (m *Manager) GetSession(guildID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[guildID]
}

// Join conecta o bot ao canal de voz de forma segura (sem Deadlock)
func (m *Manager) Join(s *discordgo.Session, guildID, channelID string) (*Session, error) {
	// 1. Verificação rápida com Lock de Leitura
	m.mu.RLock()
	if sess, ok := m.sessions[guildID]; ok {
		m.mu.RUnlock() // Libera lock antes de qualquer operação no Discord
		if sess.ChannelID != channelID {
			slog.Info("Mudando de canal", "guild_id", guildID, "old_channel", sess.ChannelID, "new_channel", channelID)
			// ChangeChannel é rápido, mas idealmente não deve bloquear o manager
			sess.Connection.ChangeChannel(channelID, false, false)
			// Atualizamos o channelID na struct (precisa de Lock de Escrita rápido)
			m.mu.Lock()
			sess.ChannelID = channelID
			m.mu.Unlock()
		}
		return sess, nil
	}
	m.mu.RUnlock()

	// 2. Conecta ao canal de voz (OPERAÇÃO LENTA E BLOQUEANTE)
	slog.Info("Conectando ao canal de voz...", "guild_id", guildID, "channel_id", channelID)
	// IMPORTANTE: Fazemos isso FORA de qualquer Lock do manager para evitar Deadlock
	// com os Event Handlers que precisam ler o manager.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return nil, err
	}

	// 3. Registra a sessão com Lock de Escrita
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verifica se outra goroutine não criou a sessão enquanto conectávamos
	if sess, ok := m.sessions[guildID]; ok {
		vc.Disconnect() // Fecha a conexão duplicada
		return sess, nil
	}

	sess := &Session{
		GuildID:        guildID,
		ChannelID:      channelID,
		Connection:     vc,
		DiscordSession: s,
	}
	m.sessions[guildID] = sess
	return sess, nil
}

// Reconnect tenta reconectar a sessão de voz existente.
// Usado quando detectamos falha no socket ou perda de conexão.
func (m *Manager) Reconnect(guildID string) error {
	sess := m.GetSession(guildID)
	if sess == nil {
		return fmt.Errorf("sessão não encontrada para reconexão")
	}

	slog.Info("Iniciando reconexão de voz...", "guild_id", guildID)

	// Tenta desconectar a conexão antiga (pode falhar se já estiver fechada)
	if sess.Connection != nil {
		sess.Connection.Disconnect()
		// Pequeno delay para limpeza
		time.Sleep(250 * time.Millisecond)
	}

	// Reconecta usando o DiscordSession armazenado
	// Usamos false, true para mute/deaf padrão
	vc, err := sess.DiscordSession.ChannelVoiceJoin(sess.GuildID, sess.ChannelID, false, true)
	if err != nil {
		return fmt.Errorf("falha ao reconectar: %w", err)
	}

	// Atualiza a referência da conexão na sessão PROTEGENDO A ESCRITA
	// Note: O Manager lock protege o MAPA e ponteiros SESS, mas modificar campos internos do Sess
	// em concorrência com PlayLoop requer cuidado.
	// O PlayLoop lê sess.Connection. Vamos assumir que a troca de ponteiro é atômica o suficiente
	// ou protegida pelo fato de que o PlayLoop está "pausado" esperando reconexão.
	
	// Mas idealmente, usaríamos um Mutex no Session também.
	// Por agora, usamos o lock global do manager para garantir exclusão mútua nessa troca crítica.
	m.mu.Lock()
	sess.Connection = vc
	m.mu.Unlock()

	// Envia silêncio para garantir handshake UDP
	if err := sendSilence(vc); err != nil {
		slog.Warn("Erro enviando silêncio na reconexão", "error", err)
	}

	slog.Info("Reconexão bem sucedida!")
	return nil
}

func (m *Manager) Leave(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[guildID]; ok {
		if sess.Cancel != nil {
			sess.Cancel()
		}
		// Disconnect pode demorar um pouco, mas no Leave é aceitável segurar o lock
		// para garantir consistência de estado imediata.
		sess.Connection.Disconnect()
		delete(m.sessions, guildID)
		slog.Info("Sessão de voz encerrada", "guild_id", guildID)
	}
}

func (sess *Session) SetLazyExit(lazy bool) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.LazyExit = lazy
}

func (sess *Session) IsLazyExit() bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.LazyExit
}

func (sess *Session) PlayLoop(filePath string, loops int) {
	if sess.Cancel != nil {
		sess.Cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess.Cancel = cancel

	go func() {
		log := slog.With("guild_id", sess.GuildID)
		defer func() {
			// Só desconecta se NÃO foi cancelado (cancelado significa que outra música começou ou comando stop foi dado mas queremos controlar o leave manualmente)
			// Na verdade, se foi cancelado por "substituição", não queremos sair.
			// Se foi cancelado por "leave", o manager já tratou.
			// Vamos verificar se o erro do contexto é Canceled.
			if ctx.Err() == context.Canceled {
				// Contexto cancelado explicitamente (provavelmente nova música tocando).
				// Não saímos do canal.
				log.Info("Playback cancelado (substituição)")
				return
			}
			
			log.Info("Playback finalizado, saindo do canal em 1s...")
			time.Sleep(1 * time.Second)
			GlobalManager.Leave(sess.GuildID)
		}()

		// 1. Aguarda conexão estar PRONTA (Ready) com Timeout
		// O handshake de voz (v4/v5) pode demorar devido ao IP Discovery e negociação de criptografia.
		timeout := time.After(10 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		ready := false
		for !ready {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				log.Warn("Timeout aguardando Voice Connection Ready INICIAL", "ready", sess.Connection.Ready)
				return
			case <-ticker.C:
				if sess.Connection.Ready {
					ready = true
				}
			}
		}

		// 2. Define falando como TRUE
		sess.Connection.Speaking(true)
		defer func() {
			// Verifica se conexão ainda existe antes de falar
			if sess.Connection != nil && sess.Connection.Ready {
				sess.Connection.Speaking(false)
			}
		}()

		// 3. Envia frames de silêncio para "aquecer" a conexão UDP e o SSRC
		if err := sendSilence(sess.Connection); err != nil {
			log.Warn("Erro enviando silêncio", "error", err)
		}

		loopCount := 0
		infinite := loops <= 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !infinite && loopCount >= loops {
					return
				}

				// Passamos a SESSÃO inteira para lidar com reconexões
				if err := playAudioFile(ctx, sess, filePath); err != nil {
					log.Error("Erro tocando áudio", "error", err, "loop", loopCount)
					// Se ocorrer erro fatal, encerra
					return
				}
				
				// Verifica Lazy Exit após terminar a música
				if sess.IsLazyExit() {
					log.Info("Lazy Exit ativado: saindo após término da música.")
					return
				}

				loopCount++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// sendSilence envia alguns pacotes de silêncio para estabelecer a prioridade RTP
func sendSilence(vc *discordgo.VoiceConnection) error {
	// 5 frames de silêncio (20ms cada) = 100ms de pre-roll
	for i := 0; i < 5; i++ {
		silenceFrame := []byte{0xF8, 0xFF, 0xFE}
		
		if !vc.Ready || vc.OpusSend == nil {
			// Se não estiver pronto, apenas retorna erro sem crashar
			return fmt.Errorf("voice connection not ready for silence")
		}
		
		vc.OpusSend <- silenceFrame
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func playAudioFile(ctx context.Context, sess *Session, filepath string) error {
	run := exec.CommandContext(ctx, "ffmpeg", "-i", filepath, "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	ffmpegOut, err := run.StdoutPipe()
	if err != nil {
		return err
	}

	if err := run.Start(); err != nil {
		return err
	}
	defer run.Wait()

	// Buffer para leitura do ffmpeg (16KB)
	ffmpegbuf := bufio.NewReaderSize(ffmpegOut, 16384)
	encoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		return fmt.Errorf("falha encoder: %v", err)
	}

	pcmBuf := make([]int16, frameSize*channels)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	// Controle de retry de conexão
	lostConnectionFrames := 0
	maxLostFrames := 500 // ~10 segundos (500 * 20ms)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// 1. Verifica estado da conexão
			// Acessamos via sess.Connection para pegar a instância mais atual (caso tenha havido reconexão)
			vc := sess.Connection
			
			if vc == nil || !vc.Ready || vc.OpusSend == nil {
				lostConnectionFrames++
				
				if lostConnectionFrames == 1 {
					slog.Warn("Conexão de voz instável/perdida. Aguardando recuperação...")
				}

				// Lógica de autoreconexão após ~3 segundos (150 frames)
				if lostConnectionFrames == 150 {
					slog.Warn("Tentando reconexão automática de voz...")
					if err := GlobalManager.Reconnect(sess.GuildID); err != nil {
						slog.Error("Falha na tentativa de reconexão", "error", err)
					} else {
						// Se reconectar com sucesso, resetamos parcialmente o contador
						// para dar tempo do handshake finalizar e vc.Ready ficar true
						lostConnectionFrames = 50 
					}
				}

				if lostConnectionFrames > maxLostFrames {
					return fmt.Errorf("timeout fatal aguardando reconexão de voz (>10s)")
				}
				
				// Sleep extra para não floodar checks
				continue
			}

			// Se recuperou de uma falha
			if lostConnectionFrames > 0 {
				slog.Info("Conexão de voz restabelecida!", "waited_frames", lostConnectionFrames)
				lostConnectionFrames = 0
			}

			// 2. Lê do FFMPEG
			err := binary.Read(ffmpegbuf, binary.LittleEndian, &pcmBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			if err != nil {
				return err
			}

			// 3. Encode Opus
			opusData, err := encoder.Encode(pcmBuf, frameSize, maxBytes)
			if err != nil {
				continue
			}

			// 4. Envia
			// Check duplo de segurança
			if vc.Ready && vc.OpusSend != nil {
				vc.OpusSend <- opusData
			}
		}
	}
}
