import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const testDirectory = path.dirname(fileURLToPath(import.meta.url));
const pluginDirectory = path.resolve(testDirectory, '..', 'com.ulanzi.commandexecutor.ulanziPlugin');
const fixturePath = path.join(testDirectory, 'fixtures', 'ulanzi-sdk-sha256.json');

async function listVendorFiles(directory, relativeDirectory = '') {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const relativePath = path.posix.join(relativeDirectory, entry.name);
    if (entry.isDirectory()) {
      files.push(...await listVendorFiles(path.join(directory, entry.name), relativePath));
    } else if (entry.isFile()) {
      files.push(relativePath);
    }
  }

  return files;
}

function parseVendorNotices(notices) {
  return notices.split('\n')
    .filter((line) => line.startsWith('| `UlanziTechnology/'))
    .map((line) => {
      const [repository, commit, sourcePathCell, license] = line.split('|')
        .slice(1, -1)
        .map((cell) => cell.trim());

      return {
        repository: repository.replaceAll('`', ''),
        commit: commit.replaceAll('`', ''),
        sourcePaths: sourcePathCell.split(', ').map((sourcePath) => sourcePath.replaceAll('`', '')),
        license,
      };
    });
}

test('fixed Ulanzi SDK vendor snapshot exactly matches its SHA-256 manifest', async () => {
  const expectedHashes = JSON.parse(await readFile(fixturePath, 'utf8'));
  const vendorRoots = [
    'plugin/vendor/ulanzi-api',
    'libs/assets',
    'libs/css',
    'libs/js',
  ];
  const actualFiles = [
    ...await Promise.all(vendorRoots.map((vendorRoot) => (
      listVendorFiles(path.join(pluginDirectory, vendorRoot), vendorRoot)
    ))),
    'LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt',
  ].flat().sort();

  assert.deepEqual(actualFiles, Object.keys(expectedHashes).sort());

  for (const relativePath of actualFiles) {
    const content = await readFile(path.join(pluginDirectory, relativePath));
    const actualHash = createHash('sha256').update(content).digest('hex');
    assert.equal(actualHash, expectedHashes[relativePath], `${relativePath} SHA-256 mismatch`);
  }

  const notices = await readFile(path.join(pluginDirectory, 'THIRD_PARTY_NOTICES.md'), 'utf8');
  assert.deepEqual(parseVendorNotices(notices), [
    {
      repository: 'UlanziTechnology/plugin-common-node',
      commit: '112bd13a7ff9d45bd68656f7e069fd61851d1812',
      sourcePaths: ['libs/constants.js', 'libs/ulanziApi.js', 'libs/utils.js'],
      license: 'Apache License 2.0',
    },
    {
      repository: 'UlanziTechnology/plugin-common-html',
      commit: '79de0b0b087546e684afd23f97223f7a7bc392da',
      sourcePaths: ['assets/', 'css/', 'js/'],
      license: 'Apache License 2.0',
    },
    {
      repository: 'UlanziTechnology/UlanziDeckPlugin-SDK',
      commit: '550ab80c69285ecf259bd494a7fff767c14f0c0f',
      sourcePaths: ['LICENSE'],
      license: 'Apache License 2.0',
    },
  ]);
});
