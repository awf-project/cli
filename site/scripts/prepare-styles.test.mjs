import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { compileString } from 'sass-embedded';
import { prepareStyles } from './prepare-styles.mjs';

test('Sass migration removes deprecations without changing the theme CSS', () => {
    // Capture expected warnings from the unmodified upstream styles only.
    const warnings = [];
    const site = fileURLToPath(new URL('../', import.meta.url));
    // Use Sass's own import resolution as an independent baseline for the adapter.
    const source = readFileSync(join(site, 'node_modules/@thulite/doks-core/assets/scss/app.scss'), 'utf8');
    const original = compileString(source, {
        loadPaths: ['assets/scss', 'node_modules/@thulite/doks-core/assets/scss', 'node_modules'].map((path) => join(site, path)),
        logger: { warn: (message) => warnings.push(message) }
    }).css;
    assert.ok(warnings.length > 0, 'Reassess the adapter when upstream no longer needs migration');
    // prepareStyles treats all active deprecations as fatal.
    assert.equal(prepareStyles(), original);
});
