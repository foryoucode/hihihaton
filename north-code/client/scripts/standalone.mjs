import fs from 'node:fs';
import path from 'node:path';

let directory = 'dist-demo';
let html = fs.readFileSync(path.join(directory, 'index.html'), 'utf8');
html = html.replace(/<script\b[^>]*src="([^"]+)"[^>]*><\/script>/g, (_, source) => {
    let script = fs.readFileSync(path.join(directory, source), 'utf8').replace(/<\/script/gi, '<\\/script');
    return '<script type="module">' + script + '</script>';
});
html = html.replace(/<link\b[^>]*href="([^"]+\.css)"[^>]*>/g, (_, source) => {
    return '<style>' + fs.readFileSync(path.join(directory, source), 'utf8') + '</style>';
});
fs.writeFileSync('north-code-demo.html', html);
console.log('Готово: north-code-demo.html — открывается без сервера и интернета.');
