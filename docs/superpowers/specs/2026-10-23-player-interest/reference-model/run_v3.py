"""Harness for the held-out test. Runs frozen-v3 (code_rules.py, score_c.py) UNCHANGED.
What this file adds, and nothing else:
 1. imports model() from frozen-v3/score_c.py (frozen-v3 first on sys.path; data files v2_raw.jsonl, tm_recs2.json and
    the v2eval player table are read as before);
 2. rule "Transfer only . Persuadable": PASS iff model()['T'] >= 10000 (the frozen decide() also accepts the loan sum,
    which this rule excludes); gate results (T is None) are FAIL; free agents (T == '?') are reported as not modelled;
 3. confidence flag (reporting only, does not change predictions): 'uncertain' when the result would differ at offered
    level L = 8 instead of the best case L = 10 (frozen code_rules.pt_value), when |T - 10000| < 15000 (uncomputed factors),
    when the player is a free agent, or when his previous club (ACTUAL_PERSON +0xd8, v3_prev.json) is FC Metz (474):
    the reason-100 gate is hardcoded by uid in the frozen scorer and is not evaluated for new players."""
import sys, json, os
here = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, here); sys.path.insert(0, os.path.join(here, 'frozen-v3'))
os.chdir(here)
import score_c
from code_rules import pt_value
assert score_c.__file__.endswith('frozen-v3/score_c.py'), score_c.__file__
uids = json.load(open('v3_uids.json')); prev = {int(k): v for k, v in json.load(open('v3_prev.json')).items()}
TH = 10000
rows = []
for name, u in uids:
    m = score_c.model(u)
    flags = []
    if m['T'] is None:
        pred, why, tot = 'FAIL', m['why'], ''
    elif m['T'] == '?':
        pred, why, tot = 'FAIL', 'free agent: path not modelled (scorer gives no transfer sum)', ''
        flags.append('free agent not modelled')
    else:
        tot = m['T']; pred = 'PASS' if tot >= TH else 'FAIL'
        why = f"T = -P_T {m['P_T']} + reason2 {m['r2']} + playing time(L=10) {m['pt10']}"
        if m['P_T'] >= 40000: why = 'just-joined penalty ' + str(m['P_T']) + '; ' + why
        r = score_c.raw[u]; o = score_c.info[u]
        pt8, _ = pt_value(r, o, 8, 4)
        t8 = -m['P_T'] + m['r2'] + (pt8 or 0)
        if (t8 >= TH) != (tot >= TH): flags.append(f'depends on offered level L (sum at L=8: {t8})')
        if abs(tot - TH) < 15000 and m['P_T'] < 40000: flags.append('near threshold; uncomputed factors')
    if prev.get(u) == 474: flags.append('previous club FC Metz: reason-100 gate not evaluated by frozen scorer')
    rows.append((name, u, pred, why, tot, 'uncertain: ' + '; '.join(flags) if flags else 'confident'))
with open('predictions-v3.tsv', 'w') as f:
    f.write('name\tuid\tprediction\treason\tscore_sum\tconfidence\n')
    for r in rows: f.write('\t'.join(str(x) for x in r) + '\n')
from collections import Counter
print(Counter(r[2] for r in rows), sum(r[5] != 'confident' for r in rows))
