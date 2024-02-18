import { Client, GatewayIntentBits, ApplicationCommandOptionType } from 'discord.js';
import dotenv from 'dotenv';
import CommandCenter from './CommandCenter';
import { Definer } from './CommandsDefiner';

dotenv.config();

const { TOKEN, CLIENT_ID } = process.env;

if (!TOKEN || !CLIENT_ID) {
	throw new Error('Variáveis de ambiente não definidas.');
}

Definer({ TOKEN, CLIENT_ID });

const client = new Client({
	intents: [
		GatewayIntentBits.Guilds,
		GatewayIntentBits.GuildMessages,
		GatewayIntentBits.GuildVoiceStates,
		GatewayIntentBits.MessageContent,
	]
});

const musicPlayer = new CommandCenter(client);

client.once('ready', () => {
	console.log(`Bot está logado como ${client.user?.tag}`);
	client.user?.setAvatar('./hakari-dance-hakari.gif')
        .then(user => console.log(`Changed profile picture for ${user.tag}`))
        .catch(console.error);
});

client.on('interactionCreate', async interaction => {
	if (!interaction.isCommand()) return;

	const { commandName } = interaction;

	if (musicPlayer.commands[commandName]) {
		try {
			musicPlayer.commands[commandName](interaction);
		} catch (error) {
			console.error(error);
			await interaction.reply('Ocorreu um erro ao processar o comando.');
		}
	}
});

client.login(TOKEN);