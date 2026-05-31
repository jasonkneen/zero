import { Glob } from 'bun';
import { mkdirSync } from 'fs';

const catalogFiles = [
  ...new Glob('src/providers/catalog/definitions/*.toml').scanSync('.'),
  ...new Glob('src/providers/catalog/models/*.toml').scanSync('.'),
];

mkdirSync('.zero-build', { recursive: true });
mkdirSync('dist', { recursive: true });

const catalogAssetEntrypoint = '.zero-build/provider-catalog-assets.ts';
await Bun.write(
  catalogAssetEntrypoint,
  catalogFiles
    .map((file, index) => {
      const importPath = `../${file.replaceAll('\\', '/')}`;
      return `import catalogAsset${index} from '${importPath}' with { type: 'file' };\nvoid catalogAsset${index};`;
    })
    .join('\n')
);

const result = await Bun.build({
  entrypoints: ['src/index.ts', catalogAssetEntrypoint],
  compile: {
    outfile: 'dist/zero',
  },
});

if (!result.success) {
  for (const log of result.logs) {
    console.error(log);
  }
  process.exit(1);
}
