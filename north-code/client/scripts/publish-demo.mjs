import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

const root = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'north-demo-'));
const env = { ...process.env, GIT_INDEX_FILE: path.join(directory, 'index') };

function git(args, input) {
    return execFileSync('git', args, { cwd: root, env, input, encoding: 'utf8' }).trim();
}

try {
    const remote = git(['ls-remote', 'origin', 'refs/heads/gh-pages']);
    let parent;
    if (remote) {
        git(['fetch', 'origin', 'gh-pages']);
        parent = git(['rev-parse', 'FETCH_HEAD']);
        git(['read-tree', parent]);
    } else {
        git(['read-tree', '--empty']);
    }

    const html = git(['hash-object', '-w', path.join(root, 'north-code/client/north-code-demo.html')]);
    const empty = git(['hash-object', '-w', '--stdin'], '');
    git(['update-index', '--add', '--cacheinfo', '100644,' + html + ',index.html']);
    git(['update-index', '--add', '--cacheinfo', '100644,' + html + ',north-code-demo.html']);
    git(['update-index', '--add', '--cacheinfo', '100644,' + empty + ',.nojekyll']);
    const tree = git(['write-tree']);
    const args = ['commit-tree', tree, '-m', 'deploy: update standalone frontend demo'];
    if (parent) args.push('-p', parent);
    const commit = git(args);
    git(['push', 'origin', commit + ':refs/heads/gh-pages']);
    console.log('Демо отправлено в gh-pages. В Settings → Pages выберите gh-pages / (root).');
} finally {
    fs.rmSync(directory, { recursive: true, force: true });
}
