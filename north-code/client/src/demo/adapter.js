import snapshot from './data.json';

const storageKey = 'north-code-demo-v1';
const copy = value => JSON.parse(JSON.stringify(value));

function initialState() {
    let data = copy(snapshot.data);
    data.dataset.id = 1;
    data.dataset.name = 'Учебный набор · демо';
    data.dataset.demo_baseline = true;
    return { active: 1, nextDataset: 2, nextPlan: 1, datasets: { 1: data }, plans: [] };
}

let state = initialState();
try {
    let saved = JSON.parse(localStorage.getItem(storageKey));
    if (saved?.datasets?.[saved.active] && Array.isArray(saved.plans)) state = saved;
} catch {}

function save() {
    try { localStorage.setItem(storageKey, JSON.stringify(state)); } catch {}
}

function fail(message, status = 400) {
    let error = new Error(message);
    error.response = { status, data: { message } };
    throw error;
}

function dataset(id) {
    let data = state.datasets[id || state.active];
    if (!data) fail('Набор данных не найден', 404);
    return data;
}

function calculate(body) {
    let data = dataset(body.dataset_id);
    if (!data.dataset.demo_baseline) {
        fail('CSV загружены для просмотра. Готовые демопланы доступны для исходного набора — нажмите «Сбросить демо», чтобы вернуться к нему.');
    }
    let budget = Number(body.budget ?? data.station[0].recommended_budget_rub);
    let deadline = Number(body.deadline ?? 14);
    if (!Number.isInteger(budget) || budget < 0 || !Number.isInteger(deadline) || deadline < 1) {
        fail('Проверьте бюджет и срок');
    }
    let disabled = [...new Set(body.disabled_suppliers || [])].sort();
    let scenarios = snapshot.scenarios.filter(item => JSON.stringify(item.disabled) === JSON.stringify(disabled));
    let scenario = scenarios.filter(item => item.deadline <= deadline).sort((a, b) => b.deadline - a.deadline)[0];
    if (!scenario) {
        fail('Для этого сочетания нет готового примера. Попробуйте срок 10, 12, 14 или 18 дней. Доступные отключения поставщиков: ' + snapshot.scenarios.filter(item => item.disabled.length).map(item => item.disabled.join(', ')).join('; '));
    }

    let plans = copy(scenario.plans);
    plans.sort((a, b) => Math.max(0, a.cost - budget) - Math.max(0, b.cost - budget)
        || b.optional_count - a.optional_count || a.cost - b.cost);
    let result = plans[0];
    result.method = body.method || 'exact';
    result.budget = budget;
    result.deadline = deadline;
    result.dataset_id = data.dataset.id;
    result.dataset_name = data.dataset.name;
    result.demo = true;
    result.optimal = false;
    result.disabled_suppliers = disabled;
    result.seconds = 0;
    if (result.cost != null) {
        result.overrun = Math.max(0, result.cost - budget);
        result.balance = Math.max(0, budget - result.cost);
    }
    result.demo_notice = 'Демо интерфейса: показан заранее рассчитанный учебный пример для срока ' + scenario.deadline
        + ' дней. Go и OR-Tools в браузере не запускаются.';
    return result;
}

function parseCSV(text) {
    let rows = [], row = [], value = '', quoted = false;
    text = text.replace(/^\ufeff/, '');
    for (let i = 0; i < text.length; i++) {
        let char = text[i];
        if (char === '"') {
            if (quoted && text[i + 1] === '"') { value += '"'; i++; }
            else quoted = !quoted;
        } else if (char === ';' && !quoted) {
            row.push(value.trim()); value = '';
        } else if ((char === '\n' || char === '\r') && !quoted) {
            if (char === '\r' && text[i + 1] === '\n') i++;
            row.push(value.trim()); value = '';
            if (row.some(item => item !== '')) rows.push(row);
            row = [];
        } else value += char;
    }
    if (quoted) fail('В CSV не закрыта кавычка');
    row.push(value.trim());
    if (row.some(item => item !== '')) rows.push(row);
    let header = rows.shift();
    if (!header || !rows.length || new Set(header).size !== header.length) fail('CSV пустой или содержит повторяющиеся столбцы');
    return rows.map(values => {
        if (values.length !== header.length) fail('Число значений не совпадает с заголовком CSV');
        return Object.fromEntries(header.map((key, i) => [key, values[i]]));
    });
}

function signature(data) {
    return JSON.stringify(['equipment', 'suppliers', 'offers', 'station'].map(name => data[name].map(row =>
        JSON.stringify(Object.fromEntries(Object.keys(row).sort().map(key => [key, String(row[key])])))).sort()));
}

async function importFiles(form) {
    let data = {};
    for (let name of ['equipment', 'suppliers', 'offers', 'station']) {
        let file = form.get(name);
        if (!file?.arrayBuffer) fail('Нужен файл ' + name);
        let text;
        try { text = new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer()); }
        catch { fail('CSV должен быть в UTF-8'); }
        data[name] = parseCSV(text);
    }
    for (let [name, key] of [['equipment', 'equipment_id'], ['suppliers', 'supplier_id'], ['offers', 'offer_id']]) {
        let ids = data[name].map(item => item[key]);
        if (ids.some(id => !id) || new Set(ids).size !== ids.length) fail('Проверьте ID в файле ' + name);
    }
    let models = new Set(data.equipment.map(item => item.equipment_id));
    let suppliers = new Set(data.suppliers.map(item => item.supplier_id));
    if (data.offers.some(item => !models.has(item.equipment_id) || !suppliers.has(item.supplier_id))) {
        fail('В предложениях есть неизвестная модель или поставщик');
    }
    if (data.station.some(item => !item.object_id || !item.required_capacity || !item.equipment_type)) {
        fail('Проверьте столбцы отчёта станции');
    }
    let id = state.nextDataset++;
    data.dataset = { id, name: String(form.get('name') || 'Импорт CSV · демо'), demo_baseline: signature(data) === signature(snapshot.data) };
    state.datasets[id] = data;
    state.active = id;
    save();
    return { dataset: data.dataset, counts: { equipment: data.equipment.length, suppliers: data.suppliers.length, offers: data.offers.length, requirements: data.station.length } };
}

export default async function adapter(config) {
    let method = (config.method || 'get').toLowerCase();
    let path = new URL(config.url, 'https://demo.local').pathname;
    let body = typeof config.data === 'string' ? JSON.parse(config.data) : config.data || {};
    let data, status = 200;

    if (method === 'get' && path === '/api/data') {
        data = dataset(config.params?.dataset_id);
    } else if (method === 'post' && path === '/api/calculate') {
        data = calculate(body);
    } else if (method === 'post' && path === '/api/search') {
        data = { equipment: dataset(body.dataset_id).equipment.filter(item => item.equipment_type === body.equipment_type
            && Number(item.capacity) >= Number(body.capacity)
            && Number(item.min_operating_temperature_c) <= Number(body.temperature)) };
    } else if (method === 'post' && path === '/api/import') {
        data = await importFiles(body);
        status = 201;
    } else if (method === 'get' && path === '/api/plans') {
        data = { plans: state.plans };
    } else if (method === 'post' && path === '/api/plans') {
        let name = String(body.name || '').trim();
        if (!name || name.length > 100) fail('Название должно содержать от 1 до 100 символов');
        let result = calculate(body);
        data = { id: state.nextPlan++, dataset_id: result.dataset_id, name, result,
            created_at: new Date().toISOString().slice(0, 19).replace('T', ' ') };
        state.plans.unshift(data);
        state.plans = state.plans.slice(0, 50);
        save();
        status = 201;
    } else if (path.startsWith('/api/plans/')) {
        let id = Number(path.split('/').pop());
        let row = state.plans.find(item => item.id === id);
        if (!row) fail('План не найден', 404);
        if (method === 'patch') {
            let name = String(body.name || '').trim();
            if (!name || name.length > 100) fail('Название должно содержать от 1 до 100 символов');
            row.name = name;
            data = row;
        } else if (method === 'delete') {
            state.plans = state.plans.filter(item => item.id !== id);
            data = { status: 'deleted' };
        } else data = row;
        save();
    } else fail('В демо этот запрос недоступен', 404);

    return { data: copy(data), status, statusText: 'OK', headers: {}, config, request: {} };
}
