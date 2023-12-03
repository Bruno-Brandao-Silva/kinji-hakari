import { REST, Routes, ApplicationCommandOptionType } from 'discord.js';

const Commands = [
    {
        name: 'jackpot',
        description: 'Kinji Hakari expande seu domínio.',
        options: [
            {
                name: 'quantas-vezes',
                description: 'Quantas vezes Kinji Hakari deve expandir seu domínio?',
                type: ApplicationCommandOptionType.Number,
            }
        ]
    },
    {
        name: 'leave',
        description: 'Kinji Hakari libera seu domínio.',
    },
];

export function Definer({ TOKEN, CLIENT_ID }: { TOKEN: string, CLIENT_ID: string }) {
    const rest = new REST({ version: '10' }).setToken(TOKEN);

    try {
        console.log('Started refreshing application (/) commands.');
        rest.put(Routes.applicationCommands(CLIENT_ID), { body: Commands }).then(() => {
            console.log('Successfully reloaded application (/) commands.');
        });
    } catch (error) {
        console.error(error);
    }
}