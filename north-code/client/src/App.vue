<script>
import axios from 'axios';

export default {
    data() {
        return {
            tab: 'station',
            method: 'exact',
            budget: 2600000,
            deadline: 14,
            disabledSuppliers: [],

            data: null,
            results: {},
            busy: false,
            loading: true,
            error: '',
            notice: '',

            history: [],
            planName: '',
            selectedObject: '',

            searchType: 'Генератор',
            searchCapacity: 8,
            searchTemperature: -30,
            models: [],
            searched: false,
            selectedModel: '',

            nav: [
                { id: 'station', name: 'Станция', icon: '⌘' },
                { id: 'purchases', name: 'План закупок', icon: '▤' },
                { id: 'catalog', name: 'Каталог', icon: '▦' },
                { id: 'experiments', name: 'Эксперименты', icon: '◈' },
                { id: 'history', name: 'История', icon: '◷' },
            ],
        };
    },
    computed: {
        result() {
            return this.results[this.method] || null;
        },

        ready() {
            return this.result?.status === 'ok';
        },

        report() {
            return this.data?.model;
        },

        types() {
            return [...new Set((this.data?.equipment || []).map((item) => item.equipment_type))];
        },

        objectDetails() {
            return (this.result?.diagnosis || []).filter((item) => item.object_id === this.selectedObject);
        },

        selectedOffers() {
            return (this.data?.offers || []).filter((item) => item.equipment_id === this.selectedModel);
        },

        groups() {
            if (!this.ready) return [];
            return [...new Set(this.result.plan.map((item) => item.supplier_id))].map((id) => ({
                ...this.data.suppliers.find((item) => item.supplier_id === id),
                items: this.result.plan.filter((item) => item.supplier_id === id),
            }));
        },

        matchedExperiments() {
            return (this.report?.experiments || []).filter((item) => item.matches_optimum).length;
        },
    },
    methods: {
        money(value) {
            return Number(value || 0).toLocaleString('ru-RU') + ' ₽';
        },

        methodName(method) {
            return method === 'exact' ? 'Точный расчёт' : 'Нейросеть';
        },

        params(method = this.method) {
            return {
                method,
                budget: Number(this.budget),
                deadline: Number(this.deadline),
                disabled_suppliers: this.disabledSuppliers,
            };
        },

        clearResults() {
            this.results = {};
            this.selectedObject = '';
            this.notice = '';
        },

        showError(error) {
            this.error =
                error.response?.data?.message || 'Не удалось выполнить запрос. Проверьте подключение к серверу.';
        },

        async calculate(compare = false) {
            if (this.busy) return;

            this.busy = true;
            this.error = '';
            this.notice = '';

            try {
                let methods = compare ? ['exact', 'neural'] : [this.method];
                for (let method of methods) {
                    let res = await axios.post('/api/calculate', this.params(method));
                    this.results[method] = res.data;
                }
            } catch (error) {
                this.showError(error);
            } finally {
                this.busy = false;
            }
        },

        async search() {
            this.error = '';

            try {
                let res = await axios.post('/api/search', {
                    equipment_type: this.searchType,
                    capacity: Number(this.searchCapacity),
                    temperature: Number(this.searchTemperature),
                });
                this.models = res.data.equipment;
                this.searched = true;
                this.selectedModel = '';
            } catch (error) {
                this.showError(error);
            }
        },

        offerInfo(offer) {
            let supplier = this.data.suppliers.find((item) => item.supplier_id === offer.supplier_id);
            let model = this.data.equipment.find((item) => item.equipment_id === offer.equipment_id);
            let ready = Number(supplier.delivery_days) + Number(model.installation_days);
            let reasons = [];
            if (Number(offer.stock_quantity) === 0) reasons.push('Нет в наличии');
            if (ready > this.deadline) reasons.push('Не успевает к сроку');
            if (this.disabledSuppliers.includes(offer.supplier_id)) reasons.push('Отключён в сценарии');
            return {
                supplier: supplier.supplier_name,
                ready,
                reason: reasons.join(' · ') || 'Подходит для одной единицы',
            };
        },

        async loadHistory() {
            try {
                let res = await axios.get('/api/plans');
                this.history = res.data.plans;
            } catch (error) {
                this.showError(error);
            }
        },

        async savePlan() {
            if (!this.result || this.busy) return;

            this.busy = true;
            this.error = '';

            try {
                await axios.post('/api/plans', {
                    ...this.params(),
                    name: this.planName.trim() || 'Сценарий · ' + this.methodName(this.method),
                });
                this.notice = 'План сохранён в историю';
                await this.loadHistory();
            } catch (error) {
                this.showError(error);
            } finally {
                this.busy = false;
            }
        },

        async renamePlan(item) {
            this.error = '';

            try {
                await axios.patch('/api/plans/' + item.id, { name: item.name });
            } catch (error) {
                this.showError(error);
                await this.loadHistory();
            }
        },

        async deletePlan(item) {
            this.error = '';

            try {
                await axios.delete('/api/plans/' + item.id);
                await this.loadHistory();
            } catch (error) {
                this.showError(error);
            }
        },

        openPlan(item) {
            this.budget = item.result.budget;
            this.deadline = item.result.deadline;
            this.disabledSuppliers = item.result.disabled_suppliers || [];
            this.method = item.result.method;
            this.results = { [this.method]: item.result };
            this.selectedObject = '';
            this.tab = 'station';
            this.notice = 'Открыт сохранённый расчёт от ' + item.created_at + ' UTC';
        },

        downloadReport() {
            let file = new Blob([JSON.stringify(this.result, null, 2)], { type: 'application/json;charset=utf-8' });
            let url = URL.createObjectURL(file);
            let link = document.createElement('a');
            link.href = url;
            link.download = 'north-code-' + this.method + '.json';
            link.click();
            URL.revokeObjectURL(url);
        },

        downloadPurchases() {
            let rows = [['Объект', 'Модель', 'Предложение', 'Поставщик', 'Количество', 'Цена', 'Готовность, дни']];
            for (let item of this.result.plan) {
                rows.push([
                    item.object_id,
                    item.equipment_id,
                    item.offer_id,
                    item.supplier_name,
                    1,
                    item.unit_price_rub,
                    item.ready_days,
                ]);
            }
            rows.push(['Оборудование', '', '', '', '', this.result.equipment_cost]);
            rows.push(['Доставка', '', '', '', '', this.result.delivery_cost]);
            rows.push(['Итого', '', '', '', '', this.result.cost]);
            let csv =
                '\ufeff' +
                rows
                    .map((row) => row.map((value) => '"' + String(value ?? '').replaceAll('"', '""') + '"').join(';'))
                    .join('\r\n');
            let url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
            let link = document.createElement('a');
            link.href = url;
            link.download = 'north-code-purchases.csv';
            link.click();
            URL.revokeObjectURL(url);
        },

        openTab(tab) {
            this.tab = tab;
            this.error = '';
            if (tab === 'history') this.loadHistory();
        },
    },
    async mounted() {
        try {
            let res = await axios.get('/api/data');
            this.data = res.data;
            this.budget = Number(this.data.station[0].recommended_budget_rub);
            this.deadline = Number(this.data.station[0].deadline_days);
            await this.calculate();
        } catch (error) {
            this.showError(error);
        } finally {
            this.loading = false;
        }
    },
};
</script>

<template>
    <div class="app-layout">
        <aside class="sidebar">
            <a class="brand" href="/" aria-label="Северный код — главная">
                <span class="brand-mark">✳</span>
                <span>
                    СЕВЕРНЫЙ
                    <br />
                    КОД
                    <span class="brand-dot">.</span>
                </span>
            </a>
            <div class="workspace-label">ЦЕНТР ВОССТАНОВЛЕНИЯ</div>
            <nav aria-label="Разделы">
                <button
                    v-for="item in nav"
                    :key="item.id"
                    :class="['nav-item', { active: tab === item.id }]"
                    @click="openTab(item.id)"
                >
                    <span class="nav-icon">{{ item.icon }}</span>
                    {{ item.name }}
                    <span v-if="tab === item.id" class="nav-arrow">↗</span>
                </button>
            </nav>
            <div class="sidebar-bottom">
                <span class="status-dot"></span>
                Станция «Северная»
                <small>ST01 · Арктическая экспедиция</small>
                <div class="sidebar-line"></div>
                <span class="small-muted">ПЛАНИРОВАНИЕ / V 1.0</span>
            </div>
        </aside>

        <main class="main-content">
            <header class="topbar">
                <span>
                    Операционный центр
                    <span class="separator">/</span>
                    {{ nav.find((item) => item.id === tab)?.name }}
                </span>
                <span class="station-code">
                    ● ST01
                    <span>СЕВЕРНАЯ</span>
                </span>
            </header>
            <div class="page-content">
                <div class="page-heading">
                    <div>
                        <div class="eyebrow">ЭКСПЕДИЦИЯ / ВОССТАНОВЛЕНИЕ ПОСЛЕ БУРИ</div>
                        <h1>
                            {{
                                tab === 'station'
                                    ? 'Вернуть станцию к жизни'
                                    : nav.find((item) => item.id === tab)?.name
                            }}
                        </h1>
                        <p>
                            {{
                                tab === 'station'
                                    ? 'Один план. Все ресурсы. Каждое решение под контролем.'
                                    : tab === 'experiments'
                                      ? 'Проверяем нейросеть на отдельных синтетических сценариях.'
                                      : tab === 'catalog'
                                        ? 'Найдите оборудование по мощности и условиям эксплуатации.'
                                        : tab === 'history'
                                          ? 'Сохранённые сценарии и результаты расчётов.'
                                          : 'Оборудование, поставщики и сроки в одном заказе.'
                            }}
                        </p>
                    </div>
                    <span class="outline-label">
                        ПОЛЯРНАЯ ПРОГРАММА
                        <span>↗</span>
                    </span>
                </div>

                <div v-if="error" class="alert error" role="alert">{{ error }}</div>
                <div v-if="notice" class="alert success" role="status">{{ notice }}</div>
                <div v-if="loading && !data" class="empty-state">Загружаем данные станции…</div>

                <template v-if="data && (tab === 'station' || tab === 'purchases')">
                    <section class="control-panel">
                        <div class="method-header">
                            <span class="section-label">СПОСОБ ПЛАНИРОВАНИЯ</span>
                            <span class="small-muted">Одинаковые данные. Разные подходы.</span>
                        </div>
                        <div class="method-grid">
                            <button
                                :disabled="busy"
                                :class="['method-card', { selected: method === 'exact' }]"
                                @click="method = 'exact'"
                            >
                                <span class="method-symbol">∑</span>
                                <span>
                                    <strong>Точный расчёт</strong>
                                    <small>Потоки · подтверждённый оптимум</small>
                                </span>
                                <span class="radio-dot"></span>
                            </button>
                            <button
                                :disabled="busy || !report"
                                :class="['method-card', { selected: method === 'neural' }]"
                                @click="method = 'neural'"
                            >
                                <span class="method-symbol">◈</span>
                                <span>
                                    <strong>
                                        Нейросеть
                                        <em>EXPERIMENTAL</em>
                                    </strong>
                                    <small>Обученная MLP · выбор закупок</small>
                                </span>
                                <span class="radio-dot"></span>
                            </button>
                        </div>
                        <fieldset :disabled="busy" class="scenario-fields">
                            <label>
                                Бюджет, ₽
                                <input
                                    type="number"
                                    v-model.number="budget"
                                    min="0"
                                    max="100000000"
                                    step="1000"
                                    @input="clearResults"
                                />
                            </label>
                            <label>
                                Срок, дней
                                <input
                                    type="number"
                                    v-model.number="deadline"
                                    min="1"
                                    max="365"
                                    @input="clearResults"
                                />
                            </label>
                            <div class="control-actions">
                                <button class="primary-btn" :disabled="busy" @click="calculate(false)">
                                    {{ busy ? 'Выполняем расчёт…' : 'Рассчитать план' }}
                                    <span v-if="!busy">↗</span>
                                </button>
                                <button class="secondary-btn" :disabled="busy || !report" @click="calculate(true)">
                                    Сравнить методы
                                </button>
                            </div>
                        </fieldset>
                        <details class="supplier-settings">
                            <summary>
                                Доступность поставщиков
                                <span>
                                    {{
                                        disabledSuppliers.length
                                            ? 'отключено: ' + disabledSuppliers.length
                                            : 'все доступны'
                                    }}
                                </span>
                            </summary>
                            <div class="supplier-checkboxes">
                                <label v-for="supplier in data.suppliers" :key="supplier.supplier_id">
                                    <input
                                        type="checkbox"
                                        :value="supplier.supplier_id"
                                        v-model="disabledSuppliers"
                                        :disabled="busy"
                                        @change="clearResults"
                                    />
                                    Отключить {{ supplier.supplier_name }}
                                </label>
                            </div>
                        </details>
                    </section>

                    <div v-if="method === 'neural'" class="alert neural-note">
                        ◈ Экспериментальная модель обучена на синтетических данных. Проверяем ограничения каждого плана,
                        но не гарантируем оптимальность. Точный решатель в этом режиме не вызывается.
                    </div>

                    <section v-if="results.exact && results.neural" class="panel compare-panel">
                        <div class="panel-heading">
                            <h2>Два метода — один сценарий</h2>
                            <span class="small-muted">Приоритеты ТЗ сохранены</span>
                        </div>
                        <div class="table-wrap">
                            <table>
                                <thead>
                                    <tr>
                                        <th>Метод</th>
                                        <th>Статус</th>
                                        <th>Мобильных пунктов</th>
                                        <th>Стоимость</th>
                                        <th>Превышение</th>
                                        <th>Время</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="key in ['exact', 'neural']" :key="key">
                                        <td>{{ methodName(key) }}</td>
                                        <td>
                                            {{
                                                results[key].status === 'ok'
                                                    ? 'План проверен'
                                                    : results[key].status === 'infeasible'
                                                      ? 'Невозможно'
                                                      : 'Не найден'
                                            }}
                                        </td>
                                        <td>{{ results[key].optional_count ?? '—' }}</td>
                                        <td>{{ results[key].cost == null ? '—' : money(results[key].cost) }}</td>
                                        <td>{{ results[key].overrun == null ? '—' : money(results[key].overrun) }}</td>
                                        <td>{{ results[key].seconds }} с</td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </section>

                    <div v-if="!result && !busy" class="empty-state">
                        Выберите метод и рассчитайте план для текущего сценария.
                    </div>
                    <div v-if="busy" class="calculating" role="status">
                        <span class="spinner"></span>
                        Проверяем потребности, остатки и варианты закупки…
                    </div>
                    <section v-if="result && !ready" class="panel">
                        <h2>{{ result.message }}</h2>
                        <p>
                            Потребности обязательных объектов, требующие закупки. При наличии подходящих предложений
                            причиной может быть общий дефицит остатков.
                        </p>
                        <ul>
                            <li v-for="item in result.uncovered" :key="item.object_id + item.equipment_type">
                                {{ item.object_id }} · {{ item.equipment_type }} — {{ item.reason }}
                            </li>
                        </ul>
                    </section>

                    <template v-if="ready">
                        <div class="result-line">
                            <span class="result-status">
                                <span class="status-dot"></span>
                                {{
                                    method === 'exact'
                                        ? 'Оптимальный план найден'
                                        : 'Нейросеть предложила допустимый план'
                                }}
                            </span>
                            <span class="small-muted">{{ result.seconds }} с · все ограничения проверены</span>
                        </div>
                        <div class="stats-grid">
                            <article class="stat-card">
                                <span>Общая стоимость</span>
                                <strong>{{ money(result.cost) }}</strong>
                                <small :class="{ 'text-danger': result.overrun > 0 }">
                                    {{ result.overrun ? 'Превышение: ' + money(result.overrun) : 'В пределах бюджета' }}
                                </small>
                            </article>
                            <article class="stat-card">
                                <span>Объекты восстановлены</span>
                                <strong>
                                    {{ result.objects.filter((item) => item.restored).length }}
                                    <i>/ {{ result.objects.length }}</i>
                                </strong>
                                <small>Все обязательные + {{ result.optional_count }} мобильных</small>
                            </article>
                            <article class="stat-card">
                                <span>Готовность станции</span>
                                <strong>
                                    {{ result.ready_days }}
                                    <i>дней</i>
                                </strong>
                                <small :class="{ 'text-warning': result.ready_days === result.deadline }">
                                    {{ result.deadline - result.ready_days }} дн. запаса до срока
                                </small>
                            </article>
                            <article class="stat-card">
                                <span>Остаток бюджета</span>
                                <strong>{{ money(result.balance) }}</strong>
                                <small>Бюджет {{ money(result.budget) }}</small>
                            </article>
                        </div>

                        <template v-if="tab === 'station'">
                            <div class="overview-grid">
                                <section class="panel station-panel">
                                    <div class="panel-heading">
                                        <div>
                                            <span class="section-label">ОБЪЕКТЫ ЭКСПЕДИЦИИ</span>
                                            <h2>Состояние станции</h2>
                                        </div>
                                        <span class="schematic-label">СХЕМА / НЕ КАРТА</span>
                                    </div>
                                    <div class="object-grid">
                                        <button
                                            v-for="item in result.objects"
                                            :key="item.object_id"
                                            :class="[
                                                'object-card',
                                                { closed: !item.restored, focused: selectedObject === item.object_id },
                                            ]"
                                            @click="
                                                selectedObject = selectedObject === item.object_id ? '' : item.object_id
                                            "
                                        >
                                            <div class="object-top">
                                                <span class="object-code">{{ item.object_id }}</span>
                                                <span class="object-icon">{{ item.optional ? '⌂' : '▥' }}</span>
                                            </div>
                                            <strong>{{ item.name }}</strong>
                                            <span class="object-tag">
                                                {{ !item.optional ? 'Обязательный объект' : 'Мобильный пункт' }}
                                            </span>
                                            <span :class="['object-state', { closed: !item.restored }]">
                                                ●
                                                {{
                                                    item.restored
                                                        ? 'Готов · день ' + item.ready_days
                                                        : 'Останется закрыт'
                                                }}
                                            </span>
                                        </button>
                                    </div>
                                    <div class="legend">
                                        <span>
                                            <i class="legend-dot green"></i>
                                            Восстановлен
                                        </span>
                                        <span>
                                            <i class="legend-dot gray"></i>
                                            Закрыт
                                        </span>
                                        <span>Нажмите на объект для диагностики ↗</span>
                                    </div>
                                </section>
                                <section class="panel budget-panel">
                                    <span class="section-label">РАСПРЕДЕЛЕНИЕ РЕСУРСОВ</span>
                                    <h2>Каждый рубль учтён</h2>
                                    <div class="budget-visual">
                                        <div
                                            class="budget-ring"
                                            :style="{
                                                background:
                                                    'conic-gradient(#d3ed83 0 ' +
                                                    (result.equipment_cost / Math.max(result.cost, result.budget)) *
                                                        100 +
                                                    '%, #648a7f 0 ' +
                                                    (result.cost / Math.max(result.cost, result.budget)) * 100 +
                                                    '%, #24363b 0 100%)',
                                            }"
                                        >
                                            <div>
                                                <span>ПОТРАЧЕНО</span>
                                                <strong>
                                                    {{ Math.round((result.cost / Math.max(1, result.budget)) * 100) }}%
                                                </strong>
                                            </div>
                                        </div>
                                    </div>
                                    <div class="budget-row">
                                        <span>
                                            <i class="legend-dot lime"></i>
                                            Оборудование
                                        </span>
                                        <strong>{{ money(result.equipment_cost) }}</strong>
                                    </div>
                                    <div class="budget-row">
                                        <span>
                                            <i class="legend-dot green"></i>
                                            Доставка
                                        </span>
                                        <strong>{{ money(result.delivery_cost) }}</strong>
                                    </div>
                                    <div class="budget-row">
                                        <span>
                                            <i class="legend-dot gray"></i>
                                            Остаток
                                        </span>
                                        <strong>{{ money(result.balance) }}</strong>
                                    </div>
                                    <p class="small-muted">
                                        Доставка оплачивается один раз каждому выбранному поставщику.
                                    </p>
                                </section>
                            </div>

                            <section v-if="selectedObject" class="panel details-panel">
                                <div class="panel-heading">
                                    <h2>Диагностика · {{ selectedObject }}</h2>
                                    <button class="text-btn" @click="selectedObject = ''">Закрыть ×</button>
                                </div>
                                <div class="table-wrap">
                                    <table>
                                        <thead>
                                            <tr>
                                                <th>Тип</th>
                                                <th>Доступно / нужно</th>
                                                <th>Состояние</th>
                                                <th>Выбранная замена</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr v-for="item in objectDetails" :key="item.key">
                                                <td>{{ item.equipment_type }}</td>
                                                <td>
                                                    {{ item.available_capacity }} / {{ item.required_capacity }}
                                                    {{ item.capacity_unit }}
                                                </td>
                                                <td>{{ item.reasons.join('; ') || 'Замена не нужна' }}</td>
                                                <td>
                                                    {{
                                                        result.plan.find((p) => p.key === item.key)?.equipment_id ||
                                                        (item.needs_replacement
                                                            ? 'Не закупается'
                                                            : 'Сохраняем установленное')
                                                    }}
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                            </section>

                            <section class="panel">
                                <div class="panel-heading">
                                    <div>
                                        <span class="section-label">СЦЕНАРНЫЙ АНАЛИЗ</span>
                                        <h2>Сколько стоит открыть больше пунктов</h2>
                                    </div>
                                    <span class="small-muted">
                                        {{
                                            method === 'exact'
                                                ? 'Минимальная стоимость для каждого количества'
                                                : 'Стоимость найденных нейросетью планов'
                                        }}
                                    </span>
                                </div>
                                <div class="frontier">
                                    <div
                                        v-for="item in result.frontier"
                                        :key="item.optional_count"
                                        :class="[
                                            'frontier-row',
                                            { chosen: item.optional_count === result.optional_count },
                                        ]"
                                    >
                                        <span>{{ item.optional_count }} из 5 пунктов</span>
                                        <div class="bar-track">
                                            <div
                                                :style="{
                                                    width:
                                                        Math.max(
                                                            2,
                                                            (item.cost /
                                                                Math.max(...result.frontier.map((row) => row.cost))) *
                                                                100,
                                                        ) + '%',
                                                }"
                                                :class="{ over: item.cost > result.budget }"
                                            ></div>
                                        </div>
                                        <strong>{{ money(item.cost) }}</strong>
                                        <small>
                                            {{
                                                item.cost > result.budget
                                                    ? '+' + money(item.cost - result.budget) + ' к бюджету'
                                                    : 'В бюджете'
                                            }}
                                        </small>
                                    </div>
                                </div>
                            </section>
                        </template>

                        <template v-if="tab === 'purchases'">
                            <section v-for="group in groups" :key="group.supplier_id" class="panel">
                                <div class="panel-heading">
                                    <div>
                                        <span class="section-label">{{ group.supplier_id }} · ПОСТАВЩИК</span>
                                        <h2>{{ group.supplier_name }}</h2>
                                    </div>
                                    <span class="supplier-badge">
                                        Доставка {{ money(group.delivery_cost_rub) }} · {{ group.delivery_days }} дн.
                                    </span>
                                </div>
                                <div class="table-wrap">
                                    <table>
                                        <thead>
                                            <tr>
                                                <th>Объект</th>
                                                <th>Оборудование</th>
                                                <th>Предложение</th>
                                                <th>Кол-во</th>
                                                <th>Цена</th>
                                                <th>Готовность</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr v-for="item in group.items" :key="item.key">
                                                <td>
                                                    <span class="object-code">{{ item.object_id }}</span>
                                                </td>
                                                <td>
                                                    <strong>{{ item.equipment_id }}</strong>
                                                    <small class="cell-detail">{{ item.model_name }}</small>
                                                </td>
                                                <td>{{ item.offer_id }}</td>
                                                <td>1</td>
                                                <td>{{ money(item.unit_price_rub) }}</td>
                                                <td>
                                                    <span
                                                        :class="[
                                                            'day-badge',
                                                            { tight: item.ready_days === result.deadline },
                                                        ]"
                                                    >
                                                        День {{ item.ready_days }}
                                                    </span>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                            </section>
                            <section class="panel">
                                <div class="panel-heading">
                                    <h2>Календарь восстановления</h2>
                                    <span class="small-muted">
                                        Доставка и установка разных устройств идут параллельно
                                    </span>
                                </div>
                                <div class="timeline">
                                    <div
                                        v-for="item in result.objects.filter((row) => row.restored)"
                                        :key="item.object_id"
                                        class="timeline-row"
                                    >
                                        <span>{{ item.object_id }}</span>
                                        <div class="bar-track">
                                            <div
                                                :style="{
                                                    width: Math.max(1, (item.ready_days / result.deadline) * 100) + '%',
                                                }"
                                            ></div>
                                        </div>
                                        <strong>День {{ item.ready_days }}</strong>
                                    </div>
                                </div>
                            </section>
                        </template>

                        <section class="save-panel">
                            <div>
                                <strong>План готов к работе</strong>
                                <p>Сохраните сценарий или выгрузите результаты.</p>
                            </div>
                            <input
                                v-model="planName"
                                maxlength="100"
                                placeholder="Название сценария"
                                aria-label="Название сценария"
                            />
                            <button class="secondary-btn" :disabled="busy" @click="savePlan">Сохранить</button>
                            <button class="secondary-btn" @click="downloadReport">JSON ↓</button>
                            <button class="primary-btn" @click="downloadPurchases">Закупки CSV ↓</button>
                        </section>
                    </template>
                </template>

                <template v-if="data && tab === 'catalog'">
                    <section class="panel">
                        <form class="catalog-search" @submit.prevent="search">
                            <label>
                                Тип оборудования
                                <select v-model="searchType">
                                    <option v-for="type in types" :key="type">{{ type }}</option>
                                </select>
                            </label>
                            <label>
                                Минимальная производительность
                                <input type="number" v-model.number="searchCapacity" min="0" required />
                            </label>
                            <label>
                                Температура, °C
                                <input type="number" v-model.number="searchTemperature" required />
                            </label>
                            <button class="primary-btn" type="submit">Найти модели ↗</button>
                        </form>
                        <p class="small-muted">
                            Поиск по характеристикам не зависит от плана восстановления. Единица производительности
                            определяется типом оборудования.
                        </p>
                    </section>
                    <section v-if="searched" class="panel">
                        <div class="panel-heading">
                            <h2>Подходящие модели</h2>
                            <span class="small-muted">Найдено: {{ models.length }}</span>
                        </div>
                        <div v-if="!models.length" class="empty-state">Нет моделей с такими характеристиками.</div>
                        <div v-else class="table-wrap">
                            <table>
                                <thead>
                                    <tr>
                                        <th>Модель</th>
                                        <th>Производительность</th>
                                        <th>Мин. температура</th>
                                        <th>Установка</th>
                                        <th></th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="item in models" :key="item.equipment_id">
                                        <td>
                                            <strong>{{ item.equipment_id }}</strong>
                                            <small class="cell-detail">{{ item.model_name }}</small>
                                        </td>
                                        <td>{{ item.capacity }} {{ item.capacity_unit }}</td>
                                        <td>{{ item.min_operating_temperature_c }} °C</td>
                                        <td>{{ item.installation_days }} дн.</td>
                                        <td>
                                            <button class="text-btn" @click="selectedModel = item.equipment_id">
                                                Предложения ↗
                                            </button>
                                        </td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </section>
                    <section v-if="selectedModel" class="panel">
                        <div class="panel-heading">
                            <h2>Предложения · {{ selectedModel }}</h2>
                            <span class="small-muted">Проверка срока: {{ deadline }} дней</span>
                        </div>
                        <div class="table-wrap">
                            <table>
                                <thead>
                                    <tr>
                                        <th>Поставщик</th>
                                        <th>Цена / шт.</th>
                                        <th>Остаток</th>
                                        <th>Цена 1 шт. с доставкой</th>
                                        <th>Готовность</th>
                                        <th>Проверка</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="item in selectedOffers" :key="item.offer_id">
                                        <td>{{ offerInfo(item).supplier }}</td>
                                        <td>{{ money(item.unit_price_rub) }}</td>
                                        <td>{{ item.stock_quantity }}</td>
                                        <td>
                                            {{
                                                money(
                                                    Number(item.unit_price_rub) +
                                                        Number(
                                                            data.suppliers.find(
                                                                (s) => s.supplier_id === item.supplier_id,
                                                            ).delivery_cost_rub,
                                                        ),
                                                )
                                            }}
                                        </td>
                                        <td>День {{ offerInfo(item).ready }}</td>
                                        <td>{{ offerInfo(item).reason }}</td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </section>
                </template>

                <template v-if="data && tab === 'experiments'">
                    <div class="alert neural-note">
                        Это воспроизводимый демонстрационный эксперимент на синтетических данных одной учебной станции.
                        Он не подтверждает качество на реальных станциях.
                    </div>
                    <div v-if="!report" class="empty-state">Модель ещё не обучена. Запустите engine/train.py.</div>
                    <template v-else>
                        <div class="stats-grid">
                            <article class="stat-card">
                                <span>Учебные сценарии</span>
                                <strong>{{ report.training_scenarios }}</strong>
                                <small>Отдельные ответы точного решателя</small>
                            </article>
                            <article class="stat-card">
                                <span>Обучающие пары</span>
                                <strong>{{ report.training_pairs.toLocaleString('ru-RU') }}</strong>
                                <small>Предпочтения между предложениями</small>
                            </article>
                            <article class="stat-card">
                                <span>Точность ранжирования</span>
                                <strong>{{ (report.validation_pair_accuracy * 100).toFixed(1) }}%</strong>
                                <small>
                                    На {{ report.validation_scenarios }} отдельных сценариях; не точность планов
                                </small>
                            </article>
                            <article class="stat-card">
                                <span>Совпадений с оптимумом</span>
                                <strong>
                                    {{ matchedExperiments }}
                                    <i>/ {{ report.experiments.length }}</i>
                                </strong>
                                <small>Полное совпадение всех приоритетов</small>
                            </article>
                        </div>
                        <section class="panel">
                            <span class="section-label">КАК ЭТО РАБОТАЕТ</span>
                            <h2>Два независимых способа выбора</h2>
                            <div class="explain-grid">
                                <div>
                                    <span class="explain-number">01 / GO · FLOW</span>
                                    <h3>Точная оптимизация</h3>
                                    <p>
                                        Перебираем наборы мобильных пунктов и поставщиков. Минимальный стоимостной
                                        поток распределяет оборудование с учётом общих остатков. Доставку считаем
                                        один раз поставщику. Алгоритм полностью работает в Go.
                                    </p>
                                </div>
                                <div>
                                    <span class="explain-number">02 / MLP</span>
                                    <h3>Обученная нейросеть</h3>
                                    <p>
                                        Сеть ранжирует покупки с учётом текущих остатков, доставки и оставшихся
                                        потребностей. Поиск с лучом ширины 8 собирает план. Перебираем наборы пунктов,
                                        проверяем планы и выбираем лучший найденный.
                                    </p>
                                </div>
                            </div>
                            <div class="architecture">{{ report.architecture }}</div>
                            <p class="small-muted">
                                Учебные seed: {{ report.training_seed_start }}–{{
                                    report.training_seed_start + report.training_scenarios - 1
                                }}. Валидация: 100000–100019. Тестовые сценарии: отдельные seed от 200000. CVAE и
                                WGAN-GP не применялись. В таблице показан офлайн-эксперимент с эталоном MILP;
                                интерактивный точный расчёт выполняется алгоритмом потоков в Go.
                            </p>
                        </section>
                        <section class="panel">
                            <div class="panel-heading">
                                <h2>Результаты на тестовых сценариях</h2>
                                <span class="small-muted">Стоимость сравнивайте после превышения и числа пунктов</span>
                            </div>
                            <div class="table-wrap">
                                <table>
                                    <thead>
                                        <tr>
                                            <th>Seed</th>
                                            <th>Бюджет / срок</th>
                                            <th>Пункты MILP / сеть</th>
                                            <th>Превышение MILP / сеть</th>
                                            <th>Стоимость MILP / сеть</th>
                                            <th>Оптимум</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <tr v-for="item in report.experiments" :key="item.seed">
                                            <td>{{ item.seed }}</td>
                                            <td>
                                                {{ money(item.budget) }}
                                                <small class="cell-detail">{{ item.deadline }} дней</small>
                                            </td>
                                            <td>
                                                {{ item.exact.optional_count ?? '—' }} /
                                                {{ item.neural.optional_count ?? '—' }}
                                            </td>
                                            <td>
                                                {{ item.exact.overrun == null ? '—' : money(item.exact.overrun) }}
                                                <small class="cell-detail">
                                                    {{ item.neural.overrun == null ? '—' : money(item.neural.overrun) }}
                                                </small>
                                            </td>
                                            <td>
                                                {{ item.exact.cost == null ? '—' : money(item.exact.cost) }}
                                                <small class="cell-detail">
                                                    {{ item.neural.cost == null ? '—' : money(item.neural.cost) }}
                                                </small>
                                            </td>
                                            <td>{{ item.matches_optimum ? 'Совпал' : 'Не совпал' }}</td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                        </section>
                    </template>
                </template>

                <template v-if="tab === 'history'">
                    <section class="panel">
                        <div class="panel-heading">
                            <h2>Сохранённые планы</h2>
                            <button class="text-btn" @click="loadHistory">Обновить ↻</button>
                        </div>
                        <div v-if="!history.length" class="empty-state">Здесь появятся сохранённые расчёты.</div>
                        <div v-else class="table-wrap">
                            <table>
                                <thead>
                                    <tr>
                                        <th>Название · редактируется</th>
                                        <th>Метод</th>
                                        <th>Стоимость</th>
                                        <th>Создано, UTC</th>
                                        <th></th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="item in history" :key="item.id">
                                        <td>
                                            <input
                                                v-model="item.name"
                                                maxlength="100"
                                                @change="renamePlan(item)"
                                                aria-label="Название сохранённого плана"
                                            />
                                        </td>
                                        <td>{{ methodName(item.result.method) }}</td>
                                        <td>{{ item.result.cost == null ? '—' : money(item.result.cost) }}</td>
                                        <td>{{ item.created_at }}</td>
                                        <td class="row-actions">
                                            <button class="text-btn" @click="openPlan(item)">Открыть ↗</button>
                                            <button class="text-btn text-danger" @click="deletePlan(item)">
                                                Удалить
                                            </button>
                                        </td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </section>
                </template>
                <footer class="page-footer">
                    <span>
                        СЕВЕРНЫЙ КОД
                        <span class="brand-dot">✳</span>
                    </span>
                    <span>Решения, которые можно проверить.</span>
                </footer>
            </div>
        </main>
    </div>
</template>
