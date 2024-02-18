import { Client, VoiceChannel, Guild, CommandInteraction, Snowflake, EmbedBuilder } from 'discord.js';
import {
    createAudioPlayer,
    joinVoiceChannel,
    createAudioResource,
    AudioPlayerStatus,
    getVoiceConnection,
} from '@discordjs/voice';

import type { Server } from './types';

class CommandCenter {   
    private client: Client;
    private servers: Record<Snowflake, Server>;

    constructor(client: Client) {
        this.client = client;
        this.servers = {};
    }

    private async leaveHandler(interaction: CommandInteraction): Promise<void> {
        if (!interaction.guildId) {
            await interaction.reply('Você precisa estar em um servidor para poder usar esse comando!');
            return;
        }

        const guildId = interaction.guildId;
        const server = this.servers[guildId];

        if (server?.connection) {
            server.connection.destroy();
            server.playerSubscription?.unsubscribe();
            this.servers[guildId] = {
                connection: null,
                channelId: null,
                playerSubscription: null,
            };
            delete this.servers[guildId];
            if (!interaction.replied)
                await interaction.reply('Kinji Hakari liberou seu domínio.');
        } else {
            await interaction.reply('Eu não estou em nenhum canal de voz!');
            return;
        }
    }

    private async playerHandler(interaction: CommandInteraction): Promise<void> {
        try {
            if (!interaction.guildId) {
                await interaction.reply('Você precisa estar em um servidor para poder usar esse comando!');
                return;
            }
            if (!interaction.member) {
                await interaction.reply('Você precisa ser membro de um servidor para poder usar esse comando!');
                return;
            }
            let opt = interaction.options.get('quantas-vezes')?.value! as number;

            if (opt <= 0) {
                await interaction.reply('Você precisa escolher um número maior que 0!');
                return;
            }
            const guild = this.client.guilds.cache.get(interaction.guildId)!
            const member = guild.members.cache.get(interaction.member.user.id)!;
            const voiceChannel = member.voice.channel!;

            if (!voiceChannel.id) {
                await interaction.reply('Você precisa estar em um canal de voz para poder usar esse comando!');
                return;
            }

            const connection = getVoiceConnection(guild.id) || joinVoiceChannel({
                channelId: voiceChannel.id,
                guildId: guild.id,
                adapterCreator: voiceChannel.guild!.voiceAdapterCreator,
            });

            const embed = new EmbedBuilder();
            embed.setTitle('Kinji Hakari expande seu domínio');
            embed.setDescription('JACKPOT!');
            embed.setColor('#7efba6');
            embed.setImage('https://media.tenor.com/Rpk3q-OLFeYAAAAC/hakari-dance-hakari.gif');

            await interaction.reply({ embeds: [embed] });

            const audioPlayer = createAudioPlayer();
            const resource = createAudioResource('./tuca-donka.mp3');

            audioPlayer.play(resource);

            const subscription = connection.subscribe(audioPlayer);

            subscription!.player.on(AudioPlayerStatus.Idle, () => {
                if (opt === undefined) {
                    setTimeout(() => audioPlayer.play(createAudioResource('./tuca-donka.mp3')), 100);
                }
                else {
                    opt--;
                    if (opt > 0) {
                        setTimeout(() => audioPlayer.play(createAudioResource('./tuca-donka.mp3')), 100);
                    } else {
                        setTimeout(() => this.leaveHandler(interaction), 5000);
                    }
                }
            });

            this.servers[guild.id] = {
                connection,
                channelId: voiceChannel.id,
                playerSubscription: subscription,
            };

            const intervalId = setInterval(() => {
                const server = this.servers[guild.id];
                if (server && server.channelId) {
                    const channel = this.client.channels.cache.get(server.channelId) as VoiceChannel | undefined;
                    if (!channel || channel.type !== 2) return;

                    const members = channel.members.size;
                    if (members === 1) {
                        setTimeout(() => {
                            const updatedMembers = channel.members.size;
                            if (updatedMembers === 1) {
                                clearInterval(intervalId);
                                this.leaveHandler(interaction);
                            }
                        }, 5000);
                    }
                }
            }, 1000);
        } catch (error: any) {
            console.error(error);
        }
    }

    public commands: { [command: string]: (interaction: CommandInteraction) => void } = {
        jackpot: async (interaction) => this.playerHandler(interaction),
        leave: (interaction) => this.leaveHandler(interaction)
    };
}

export default CommandCenter;
