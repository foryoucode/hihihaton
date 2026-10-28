from collections import Counter
from pathlib import Path

import numpy as np

from planner import assignment, selected_needs


MODEL_PATH = Path(__file__).parent / 'models' / 'ranker.npz'
FEATURES = ['price', 'incremental_cost', 'delivery_cost', 'delivery_paid', 'time_slack',
            'capacity_surplus', 'temperature_margin', 'stock', 'supplier_coverage', 'relative_price']


def features(req, offers, used, remaining, candidates, deadline):
    cheapest = min(p['unit_price_rub'] for p in offers)
    result = []
    for p in offers:
        paid = p['supplier_id'] in used
        shipping = int(p['supplier']['delivery_cost_rub'])
        coverage = sum(any(q['supplier_id'] == p['supplier_id'] for q in candidates[r['key']]) for r in remaining)
        result.append([p['unit_price_rub'] / 1e6,
                       (p['unit_price_rub'] + (0 if paid else shipping)) / 1e6,
                       shipping / 2e5, float(paid), (deadline - p['ready_days']) / 14,
                       min(3, float(p['model']['capacity']) / float(req['required_capacity']) - 1),
                       (float(req['operating_temperature_c']) - float(p['model']['min_operating_temperature_c'])) / 50,
                       p['stock_quantity'] / 8, coverage / max(1, len(remaining)),
                       p['unit_price_rub'] / max(1, cheapest) - 1])
    return np.asarray(result)


class Ranker:
    def __init__(self, seed=42):
        rng = np.random.default_rng(seed)
        self.w1 = rng.normal(0, 0.15, (len(FEATURES), 24))
        self.b1 = np.zeros(24)
        self.w2 = rng.normal(0, 0.15, (24, 1))
        self.b2 = np.zeros(1)

    def score(self, x):
        return (np.tanh(x @ self.w1 + self.b1) @ self.w2 + self.b2).ravel()

    def fit(self, positive, negative, epochs=160, seed=42):
        rng = np.random.default_rng(seed)
        params = [self.w1, self.b1, self.w2, self.b2]
        first = [np.zeros_like(p) for p in params]
        second = [np.zeros_like(p) for p in params]
        step = 0
        for epoch in range(epochs):
            indexes = rng.permutation(len(positive))
            for start in range(0, len(indexes), 256):
                idx = indexes[start:start + 256]
                xp, xn = positive[idx], negative[idx]
                hp = np.tanh(xp @ self.w1 + self.b1)
                hn = np.tanh(xn @ self.w1 + self.b1)
                diff = (hp @ self.w2 - hn @ self.w2).ravel()
                grad = -1 / (1 + np.exp(np.clip(diff, -40, 40))) / len(idx)
                gp = (grad[:, None] @ self.w2.T) * (1 - hp ** 2)
                gn = (-grad[:, None] @ self.w2.T) * (1 - hn ** 2)
                gradients = [xp.T @ gp + xn.T @ gn, (gp + gn).sum(axis=0),
                             (hp - hn).T @ grad[:, None], np.zeros(1)]
                step += 1
                for i, (param, g) in enumerate(zip(params, gradients)):
                    g = g + 0.0001 * param
                    first[i] = 0.9 * first[i] + 0.1 * g
                    second[i] = 0.999 * second[i] + 0.001 * g ** 2
                    param -= 0.003 * (first[i] / (1 - 0.9 ** step)) / (np.sqrt(second[i] / (1 - 0.999 ** step)) + 1e-8)
        return float(np.logaddexp(0, -(self.score(positive) - self.score(negative))).mean())

    def save(self, path=MODEL_PATH):
        np.savez(path, w1=self.w1, b1=self.b1, w2=self.w2, b2=self.b2)

    @classmethod
    def load(cls, path=MODEL_PATH):
        model = cls()
        with np.load(path, allow_pickle=False) as data:
            for key in ['w1', 'b1', 'w2', 'b2']:
                setattr(model, key, data[key])
        return model


def neural_subset(needs, candidates, selected, deadline, model, width=8):
    required = selected_needs(needs, selected)
    if any(not candidates[r['key']] for r in required):
        return None
    states = [(0.0, [], set(), Counter())]
    for i, req in enumerate(required):
        next_states = []
        for score, plan, used, counts in states:
            offers = [p for p in candidates[req['key']] if counts[p['offer_id']] < p['stock_quantity']]
            if not offers:
                continue
            x = features(req, offers, used, required[i + 1:], candidates, deadline)
            scores = model.score(x)
            log_probs = scores - np.logaddexp.reduce(scores)
            for index in np.argsort(scores)[-width:]:
                p = offers[index]
                new_counts = counts.copy()
                new_counts[p['offer_id']] += 1
                next_states.append((score + float(log_probs[index]), plan + [assignment(req, p)],
                                    used | {p['supplier_id']}, new_counts))
        if not next_states:
            return None
        states = sorted(next_states, key=lambda state: state[0], reverse=True)[:width]
    return states[0][1]
