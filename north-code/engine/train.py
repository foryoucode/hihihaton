import argparse
import hashlib
import json
import time
from pathlib import Path

import numpy as np

from neural import FEATURES, MODEL_PATH, Ranker, features
from planner import exact_subset, load_data, make_scenario, prepare, selected_needs, solve


def collect(base, seeds):
    positive, negative = [], []
    solved = 0

    for seed in seeds:
        data = make_scenario(base, seed)
        deadline = int(data['station'][0]['deadline_days'])
        needs, candidates = prepare(data, deadline)
        rng = np.random.default_rng(seed)
        optional = sorted({r['object_id'] for r in data['station'] if r['optional'] == 'yes'})
        selected = [key for key in optional if rng.random() > 0.4]
        plan = exact_subset(needs, candidates, selected)

        if plan is None:
            continue

        solved += 1
        chosen = {r['key']: r['offer_id'] for r in plan}
        used = set()
        counts = {}
        required = selected_needs(needs, selected)

        for i, req in enumerate(required):
            offers = [p for p in candidates[req['key']] if counts.get(p['offer_id'], 0) < p['stock_quantity']]
            x = features(req, offers, used, required[i + 1:], candidates, deadline)
            index = next(j for j, p in enumerate(offers) if p['offer_id'] == chosen[req['key']])

            for j in range(len(offers)):
                if j != index:
                    positive.append(x[index])
                    negative.append(x[j])

            p = offers[index]
            used.add(p['supplier_id'])
            counts[p['offer_id']] = counts.get(p['offer_id'], 0) + 1

    return np.asarray(positive), np.asarray(negative), solved


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--scenarios', type=int, default=180)
    parser.add_argument('--test-scenarios', type=int, default=8)
    args = parser.parse_args()

    if args.scenarios < 10 or args.test_scenarios < 1:
        parser.error('Нужно минимум 10 учебных сценариев и один тестовый')

    base = load_data()
    print('Собираем учебные сценарии и ответы MILP...', flush=True)
    xp, xn, solved = collect(base, range(1000, 1000 + args.scenarios))
    vp, vn, valid_solved = collect(base, range(100000, 100020))

    if len(xp) == 0 or len(vp) == 0:
        raise RuntimeError('Нет допустимых учебных примеров')

    model = Ranker()
    loss = model.fit(xp, xn)
    MODEL_PATH.parent.mkdir(exist_ok=True)
    model.save()
    report = {'architecture': 'MLP 10 → 24 tanh → 1; pairwise ranking, Adam; beam search width 8',
              'seed': 42, 'training_seed_start': 1000, 'training_scenarios': args.scenarios,
              'training_feasible': solved, 'training_pairs': len(xp), 'training_loss': loss,
              'validation_scenarios': 20, 'validation_feasible': valid_solved,
              'validation_pair_accuracy': float(np.mean(model.score(vp) > model.score(vn))),
              'features': FEATURES,
              'limitations': 'Синтетические сценарии на структуре одной учебной станции. Это демонстрационный эксперимент, не оценка качества на реальных станциях. CVAE и WGAN-GP не использовались.',
              'model_sha256': hashlib.sha256(MODEL_PATH.read_bytes()).hexdigest(), 'experiments': []}

    for seed in range(200000, 200000 + args.test_scenarios):
        data = make_scenario(base, seed)
        budget = int(data['station'][0]['recommended_budget_rub'])
        deadline = int(data['station'][0]['deadline_days'])
        row = {'seed': seed, 'budget': budget, 'deadline': deadline}

        for method in ['exact', 'neural']:
            start = time.perf_counter()
            result = solve(data, budget, deadline, method, model)
            row[method] = {key: result.get(key) for key in ['status', 'cost', 'overrun', 'optional_count', 'validated']}
            row[method]['seconds'] = round(time.perf_counter() - start, 4)

        exact, neural = row['exact'], row['neural']
        row['matches_optimum'] = exact['status'] == neural['status'] == 'ok' and all(
            exact[k] == neural[k] for k in ['overrun', 'optional_count', 'cost'])
        report['experiments'].append(row)
        print('Тестовый сценарий', seed, row['matches_optimum'], flush=True)

    path = Path(__file__).parent / 'models' / 'metrics.json'
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print('Сохранены модель и отчёт:', path, flush=True)


if __name__ == '__main__':
    main()
