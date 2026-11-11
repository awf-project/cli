import { spawn } from 'node:child_process';
import { watch } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { prepareStyles } from './prepare-styles.mjs';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
prepareStyles();
const args = process.argv.slice(2);
const hugo = spawn('hugo', args, { stdio: 'inherit' });
let watcher;
let timer;
if (args[0] === 'server') {
    watcher = watch('assets/scss', { recursive: true }, () => {
        clearTimeout(timer);
        timer = setTimeout(() => {
            try {
                prepareStyles();
            } catch (error) {
                console.error(error);
            }
        }, 100);
    });
}
for (const signal of ['SIGINT', 'SIGTERM']) {
    process.on(signal, () => hugo.kill(signal));
}
hugo.on('error', (error) => {
    console.error(error.message);
    process.exit(1);
});
hugo.on('exit', (code, signal) => {
    clearTimeout(timer);
    watcher?.close();
    process.exit(signal === 'SIGINT' ? 0 : (code ?? 1));
});
