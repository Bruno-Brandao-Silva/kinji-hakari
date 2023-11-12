import { Client, VoiceChannel, Guild, Message } from 'discord.js';
import {
    createAudioPlayer,
    joinVoiceChannel,
    createAudioResource,
    AudioPlayerStatus,
    getVoiceConnection,
} from '@discordjs/voice';

import type { Server } from './types';

const isMemberInVoiceChannel = (message: Message): asserts message => {
    if (!message.member) {
        throw new Error('Você precisa ser membro de um servidor para poder me tirar!');
    }
    if (!message.member.voice.channel || !message.guild) {
        throw new Error('Você precisa estar em um canal de voz para poder me tirar!');
    }
};

const playerHandler = async (client: Client, message: Message, servers: Record<string, Server>, loop = false) => {
    try {
        // isMemberInVoiceChannel(message);
        const voiceChannel = message.member!.voice.channel!;
        const guild = message.guild as Guild;
        const connection = getVoiceConnection(guild.id) || joinVoiceChannel({
            channelId: voiceChannel.id,
            guildId: guild.id,
            adapterCreator: voiceChannel.guild.voiceAdapterCreator,
        });

        await message.channel.send('JACKPOT!');
        await message.channel.send('https://tenor.com/gdXo1Jz9Bd4.gif');

        const audioPlayer = createAudioPlayer();
        const resource = createAudioResource('./tuca-donka.mp3');

        audioPlayer.play(resource);

        const subscription = connection.subscribe(audioPlayer);

        subscription!.player.on(AudioPlayerStatus.Idle, () => {
            if (loop) {
                setTimeout(() => audioPlayer.play(createAudioResource('./tuca-donka.mp3')), 100);
            } else {
                setTimeout(() => leaveHandler(voiceChannel.id, guild.id, servers), 5000);
            }
        });

        servers[guild.id] = {
            connection,
            channelId: voiceChannel.id,
            playerSubscription: subscription,
        };

        const intervalId = setInterval(() => {
            const server = servers[guild.id];
            if (server && server.channelId) {
                const channel = client.channels.cache.get(server.channelId) as VoiceChannel | undefined;
                if (!channel || channel.type !== 2) return;

                const members = channel.members.size;
                if (members === 1) {
                    setTimeout(() => {
                        const updatedMembers = channel.members.size;
                        if (updatedMembers === 1) {
                            clearInterval(intervalId);
                            leaveHandler(channel.id, guild.id, servers);
                        }
                    }, 5000);
                }
            }
        }, 1000);
    } catch (error: any) {
        await message.reply(error.message);
    }
};

const leaveHandler = async (voiceChannelId: string, guildId: string, servers: Record<string, Server>) => {
    const server = servers[guildId];
    if (server?.connection) {
        server.connection.destroy();
        server.playerSubscription?.unsubscribe();
        servers[guildId] = {
            connection: null,
            channelId: null,
            playerSubscription: null,
        };
        delete servers[guildId];
    } else {
        throw new Error('Eu não estou em nenhum canal de voz!');
    }
};

const commands: { [command: string]: (client: Client<boolean>, message: Message<boolean>, servers: { [guildId: string]: Server }) => void } = {
    JACKPOT: async (client, message, servers) => playerHandler(client, message, servers, true),
    jackpot: async (client, message, servers) => playerHandler(client, message, servers),
    leave: (client, message, servers) => {
        const voiceChannelId = message.member?.voice.channelId;
        const server = servers[message.guildId!];

        if (!server || server.channelId !== voiceChannelId) {
            throw new Error('Você precisa estar no mesmo canal de voz que eu para poder me tirar!');
        }
        leaveHandler(voiceChannelId!, message.guildId!, servers);
    },
};
commands.JACKPOT = async (client, message, servers) => playerHandler(client, message, servers, true);
export default commands;