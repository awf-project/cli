import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { compile, deprecations } from 'sass-embedded';

const site = fileURLToPath(new URL('../', import.meta.url));
const roots = ['assets/scss', 'node_modules/@thulite/doks-core/assets/scss', 'node_modules'].map((path) => join(site, path));

// Doks and Bootstrap still share global variables across legacy Sass imports.
// Inline these imports in order before migrating: converting each file to @use
// independently introduces circular dependencies and changes override semantics.
function inlineImports(file, ancestors = []) {
    if (ancestors.includes(file)) throw new Error(`Circular Sass import: ${file}`);
    return readFileSync(file, 'utf8').replace(/^\s*@import ["']([^"']+)["'];[^\n]*/gm, (statement, name) => {
        if (/^(https?:|url\()/.test(name) || name.endsWith('.css')) return statement;
        for (const root of [dirname(file), ...roots]) {
            const path = join(root, name);
            for (const candidate of [path + '.scss', join(dirname(path), '_' + basename(path) + '.scss')]) {
                if (existsSync(candidate)) return inlineImports(candidate, [...ancestors, file]);
            }
        }
        throw new Error(`Cannot resolve Sass import ${name} from ${file}`);
    });
}

function legacyStyles() {
    return inlineImports(join(roots[1], 'app.scss'));
}

export function prepareStyles() {
    const output = join(site, '.generated');
    mkdirSync(join(output, 'css'), { recursive: true });
    const source = join(output, 'app.scss');
    writeFileSync(source, legacyStyles());
    const migrator = join(site, 'node_modules/sass-migrator/sass-migrator.js');
    for (const args of [['module', '--built-in-only'], ['color'], ['if-function']]) {
        execFileSync(process.execPath, [migrator, ...args, 'app.scss'], { cwd: output, stdio: 'inherit' });
    }
    // Legacy red()/green()/blue() rounded to integers. color.channel() preserves
    // fractions; retain the old behavior for Bootstrap's luminance lookup table.
    writeFileSync(source, readFileSync(source, 'utf8').replace(/color\.channel\([^()]+, \$space: rgb\)/g, 'math.round($&)'));
    const css = compile(source, {
        style: 'expanded',
        // Treat every active deprecation as a failure instead of hiding warnings.
        fatalDeprecations: Object.values(deprecations)
            .filter((deprecation) => deprecation.status === 'active')
            .map((deprecation) => deprecation.id)
    }).css;
    const target = join(output, 'css/app.css');
    if (!existsSync(target) || readFileSync(target, 'utf8') !== css) writeFileSync(target, css);
    return css;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) prepareStyles();
