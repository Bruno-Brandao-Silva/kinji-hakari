import { ChatInputCommandInteraction, SlashCommandBuilder } from "discord.js";
import { VoiceConnectionManager } from "../models/VoiceConnectionManager";

export const LeaveCommandJSON = new SlashCommandBuilder()
    .setName('leave')
    .setDescription('Kinji Hakari libera seu domínio.')
    .toJSON();

export const LeaveCommand = (interaction: ChatInputCommandInteraction) => {
    if (!interaction.guildId) {
        interaction.reply('Você precisa estar em um servidor para poder usar esse comando!');
        return;
    }

    if (!interaction.member) {
        interaction.reply('Você precisa ser membro de um servidor para poder usar esse comando!');
        return;
    }

    const manager = VoiceConnectionManager.getConnection(interaction.guildId);
    if (manager) {
        interaction.reply('Kinji Hakari liberou seu domínio.');
        VoiceConnectionManager.deleteConnection(interaction.guildId);
        return;
    } else {
        interaction.reply('Eu não estou em nenhum canal de voz!');
        return;
    }
};