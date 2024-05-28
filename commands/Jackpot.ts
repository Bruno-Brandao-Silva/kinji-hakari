import {
    ChatInputCommandInteraction,
    GuildMember,
    SlashCommandBuilder,
    EmbedBuilder,
} from "discord.js";
import {
    createAudioPlayer,
    joinVoiceChannel,
    createAudioResource,
    AudioPlayerStatus,
    VoiceConnectionStatus,
} from '@discordjs/voice';
import { VoiceConnectionManager, VoiceConnectionType } from "../models/VoiceConnectionManager";

const embed = new EmbedBuilder()
    .setTitle('Kinji Hakari expande seu domínio')
    .setDescription('JACKPOT!')
    .setColor('#7efba6')
    .setImage('https://media.tenor.com/Rpk3q-OLFeYAAAAC/hakari-dance-hakari.gif');

export const JackpotCommandJSON = new SlashCommandBuilder()
    .setName('jackpot')
    .setDescription('Kinji Hakari expande seu domínio.')
    .addNumberOption(option =>
        option.setName('quantas-vezes')
            .setDescription('Quantas vezes Kinji Hakari deve expandir seu domínio?')
            .setRequired(false))
    .toJSON();

export const JackpotCommand = (interaction: ChatInputCommandInteraction) => {
    try {
        const { guildId, member } = interaction;

        if (!guildId) {
            interaction.reply('Você precisa estar em um servidor para poder usar esse comando!');
            return;
        }

        if (!member || !(member instanceof GuildMember)) {
            interaction.reply('Você precisa ser membro de um servidor para poder usar esse comando!');
            return;
        }
        
        if (!member.voice.channel) {
            interaction.reply('Você precisa estar em um canal de voz para poder usar esse comando!');
            return;
        }

        const voiceChannel = member.voice.channel;
        const opt = interaction.options.getNumber('quantas-vezes', false);
        if (opt && opt <= 0) {
            interaction.reply('Você precisa escolher um número maior que 0!');
            return;
        }



        const existingManager = VoiceConnectionManager.getConnection(guildId);
        const connection = existingManager?.connection || joinVoiceChannel({
            channelId: voiceChannel.id,
            guildId,
            adapterCreator: voiceChannel.guild.voiceAdapterCreator,
        });

        if (existingManager?.connection && existingManager.channelId !== voiceChannel.id) {
            interaction.reply('Hakari já está em outro canal de voz!');
            return;
        }

        interaction.reply({ embeds: [embed] });

        const audioPlayer = existingManager?.audioPlayer || createAudioPlayer();
        connection.subscribe(audioPlayer);

        const playAudio = () => {
            const resource = createAudioResource('./tuca-donka.mp3', { inlineVolume: true });
            resource.volume?.setVolume(0.35);
            audioPlayer.play(resource);
        };

        let playCount = 0;
        audioPlayer.state.status == AudioPlayerStatus.Idle && playAudio();

        audioPlayer.removeAllListeners();
        audioPlayer.on(AudioPlayerStatus.Idle, () => {
            if (!opt || ++playCount < opt) {
                setTimeout(playAudio, 100);
            } else {
                setTimeout(() => VoiceConnectionManager.deleteConnection(guildId), 5000);
            }
        });

        const intervalId = existingManager?.intervalId || setInterval(() => {
            if (!connection || connection.state.status === VoiceConnectionStatus.Destroyed) {
                clearInterval(intervalId);
                return;
            }

            if (voiceChannel.members.size === 1) {
                setTimeout(() => {
                    if (voiceChannel.members.size === 1) {
                        VoiceConnectionManager.deleteConnection(guildId);
                    }
                }, 5000);
            }
        }, 1000);
        VoiceConnectionManager.setConnection(guildId, voiceChannel.id, intervalId, connection, audioPlayer);
    } catch (error) {
        console.error(error);
    }
};
